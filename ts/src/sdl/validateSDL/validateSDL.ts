import type { ErrorMessages, ValidationError, ValidationFunction } from "../../utils/jsonSchemaValidation.ts";
import { dirname, getErrorLocation, humanizeErrors } from "../../utils/jsonSchemaValidation.ts";
import { MAX_HTTP_TIMEOUT_MILLISECONDS, parseHTTPTimeout } from "../httpTimeout.ts";
import { compareStrings, INTERCONNECT_GROUP_AUTO, parseStorageBytes, resolveInterconnectGroup } from "../manifest/manifestUtils.ts";
import { declaredVolumeNames, isBucketVolume, isExternalVolume, serviceVolumeNames } from "../manifest/volumes.ts";
import { castArray, stringToBoolean } from "../utils.ts";
import { schema as validationSDLSchema, type SDLInput, validate as validateSDLInput, type VolumeDeclaration } from "./validateSDLInput.ts";

export { validationSDLSchema };
export type { SDLInput };

export interface SDLOptions {
  /**
   * Deployment owner (the tx signer, bech32). SDL 2.2 volume references
   * (external, adopt, replica-of) compile to owner-keyed VolumeRefs, and the
   * SDL omits the owner because it is the signer; an explicit
   * `volumes.<name>.external.owner` wins.
   */
  owner?: string;
}

const ERROR_MESSAGES: ErrorMessages = {
  "#/definitions/storageRamClassMustNotBePersistent"(error) {
    return `"ram" storage${getErrorLocation(dirname(error.instancePath))} cannot be persistent`;
  },
  "#/definitions/exposeToWithIpEnforcesGlobal"() {
    return `If an IP is declared, the directive must be declared as global.`;
  },
  // Mirrors the shared schema's `min_window` pattern (`^[1-9][0-9]*(s|m|h)$`).
  // Go stays the lenient layer (`go/sdl/reclamation.go` accepts any `> 0`
  // `time.ParseDuration`), so this is a sanctioned schema-only-stricter rule.
  "#/properties/reclamation/properties/min_window/pattern"() {
    return `Reclamation min_window must be a whole number followed by s, m, or h (e.g. "24h", "30m").`;
  },
  "#/definitions/legacyVersionConstraints/then/not"() {
    return `The "volumes" section requires SDL version 2.2.`;
  },
  "#/definitions/legacyVersionConstraints/if": reportedByThenBranch,
  "#/definitions/v2_2RequiresServicesOrVolumes/if": reportedByThenBranch,
};

function reportedByThenBranch(): undefined {
  return undefined;
}

export function validateSDL(sdl: SDLInput, options: SDLOptions = {}): undefined | ValidationError[] {
  validateSDLInput(sdl);
  const schemaErrors = humanizeErrors((validateSDLInput as ValidationFunction).errors, validationSDLSchema, ERROR_MESSAGES);
  if (schemaErrors.length) return schemaErrors;

  const validator = new SDLValidator(sdl, options);
  const errors = validator.validate();

  const allErrors = schemaErrors.concat(errors);
  return allErrors.length ? allErrors : undefined;
}

class SDLValidator {
  readonly #endpointsUsed = new Set<string>();
  readonly #portsUsed = new Map<string, string>();
  readonly #teeTypeByPlacement = new Map<string, string>();
  readonly #volumesUsed = new Set<string>();
  readonly #sdl: SDLInput;
  readonly #owner: string | undefined;
  readonly #errors: ValidationError[] = [];

  constructor(sdl: SDLInput, options: SDLOptions) {
    this.#sdl = sdl;
    this.#owner = options.owner || undefined;
  }

  validate() {
    if (this.#sdl.services) {
      Object.keys(this.#sdl.services).forEach((serviceName) => {
        this.#validateDeploymentWithRelations(serviceName);
        this.#validateLeaseIP(serviceName);
        this.#validateHTTPTimeoutRanges(serviceName);
      });
    }

    if (this.#sdl.version === "2.2") {
      this.#validateVolumes();
    } else {
      this.#validateNoVolumeReferences();
    }

    this.#validateInterconnect();
    this.#validateEndpoints();
    return this.#errors;
  }

  // JSON Schema caps numeric values, but its string pattern cannot enforce the
  // maximum after unit conversion. For example, "1194h" matches the pattern.
  #validateHTTPTimeoutRanges(serviceName: string) {
    const exposes = this.#sdl.services?.[serviceName]?.expose ?? [];
    for (const [exposeIndex, expose] of exposes.entries()) {
      const options = expose.http_options;
      if (!options) continue;

      for (const field of ["read_timeout", "send_timeout"] as const) {
        const value = options[field];
        if (value === undefined) continue;

        const result = parseHTTPTimeout(value);
        if (result.ok) continue;

        this.#errors.push({
          message: `HTTP ${field.replace("_", " ")} ${result.message}.`,
          instancePath: `/services/${serviceName}/expose/${exposeIndex}/http_options/${field}`,
          schemaPath: "#/definitions/httpTimeout/maximum",
          keyword: "maximum",
          params: {
            comparison: "<=",
            limit: MAX_HTTP_TIMEOUT_MILLISECONDS,
          },
        });
      }
    }
  }

  #validateDeploymentWithRelations(serviceName: string) {
    const deployment = this.#sdl.deployment[serviceName];
    if (!deployment) {
      this.#errors.push({
        message: `Service "${serviceName}" is not defined at "/deployment" section.`,
        instancePath: `/deployment`,
        schemaPath: "#/properties/deployment",
        keyword: "required",
        params: {
          missingProperty: serviceName,
        },
      });
      return;
    }

    Object.keys(this.#sdl.deployment[serviceName]).forEach((deploymentName) => {
      this.#validateDeploymentRelations(serviceName, deploymentName);
      this.#validateServiceStorages(serviceName, deploymentName);
      this.#validateStorages(serviceName, deploymentName);
      this.#validateGPU(serviceName, deploymentName);
      this.#validateTEE(serviceName, deploymentName);
    });
  }

  #validateDeploymentRelations(serviceName: string, deploymentName: string) {
    const serviceDeployment = this.#sdl.deployment?.[serviceName]?.[deploymentName];
    const compute = this.#sdl.profiles?.compute?.[serviceDeployment?.profile];

    this.#validatePlacementPricing(deploymentName, serviceDeployment?.profile);

    if (!compute) {
      this.#errors.push({
        message: `The compute requirements for the "${serviceDeployment?.profile}" profile are not defined in the "compute" section.`,
        instancePath: `/profiles/compute`,
        schemaPath: "#/properties/profiles/properties/compute",
        keyword: "required",
        params: {
          missingProperty: serviceDeployment?.profile,
        },
      });
    }
  }

  #validatePlacementPricing(placementName: string, profileName: string) {
    const infra = this.#sdl.profiles?.placement?.[placementName];

    if (!infra) {
      this.#errors.push({
        message: `The placement "${placementName}" is not defined in the "placement" section.`,
        instancePath: `/profiles/placement`,
        schemaPath: "#/properties/profiles/properties/placement",
        keyword: "required",
        params: {
          missingProperty: placementName,
        },
      });
    }

    if (infra && !infra.pricing?.[profileName]) {
      this.#errors.push({
        message: `The pricing for the "${profileName}" profile is not defined in the "${placementName}" placement.`,
        instancePath: `/profiles/placement/${placementName}/pricing`,
        schemaPath: "#/properties/profiles/properties/placement/additionalProperties/properties/pricing",
        keyword: "required",
        params: {
          missingProperty: profileName,
        },
      });
    }
  }

  #validateServiceStorages(serviceName: string, deploymentName: string) {
    const service = this.#sdl.services?.[serviceName];
    const mounts: Record<string, string> = {};
    const serviceDeployment = this.#sdl.deployment[serviceName][deploymentName];
    const compute = this.#sdl.profiles?.compute?.[serviceDeployment.profile];
    const storages = castArray(compute?.resources.storage);

    if (!service?.params?.storage) {
      return;
    }

    Object.entries(service.params.storage).forEach(([storageName, storage]) => {
      if (!storage) {
        this.#errors.push({
          message: `Storage "${storageName}" is not configured.`,
          instancePath: `/services/${serviceName}/params/storage/${storageName}`,
          schemaPath: "#/properties/services/additionalProperties/properties/params/properties/storage/additionalProperties",
          keyword: "required",
          params: {
            missingProperty: storageName,
          },
        });
        return;
      }
      const storageNameExists = storages.some(({ name }) => name === storageName);
      if (!storage.volume && !storageNameExists) {
        this.#errors.push({
          message: `Service "${serviceName}" references non-existing compute volume "${storageName}".`,
          instancePath: `/profiles/compute/${serviceDeployment.profile}/resources/storage`,
          schemaPath: "#/properties/profiles/properties/compute/additionalProperties/properties/resources/properties/storage",
          keyword: "required",
          params: {
            missingProperty: storageName,
          },
        });
        return;
      }

      const mount = String(storage.mount);
      const volumeName = mounts[mount];

      if (volumeName && !storage.mount) {
        this.#errors.push({
          message: "Multiple root ephemeral storages are not allowed.",
          instancePath: `/services/${serviceName}/params/storage/${storageName}`,
          schemaPath: "#/properties/services/additionalProperties/properties/params/properties/storage",
          keyword: "uniqueItems",
          params: {
            duplicate: volumeName,
          },
        });
      }
      if (volumeName && storage.mount) {
        this.#errors.push({
          message: `Mount "${mount}" already in use by volume "${volumeName}".`,
          instancePath: `/services/${serviceName}/params/storage/${storageName}/mount`,
          schemaPath: "#/properties/services/additionalProperties/properties/params/properties/storage/additionalProperties/properties/mount",
          keyword: "uniqueItems",
          params: {
            duplicate: mount,
          },
        });
      }

      mounts[mount] = storageName;
    });
  }

  #validateStorages(serviceName: string, deploymentName: string) {
    const service = this.#sdl.services?.[serviceName];
    const serviceDeployment = this.#sdl.deployment[serviceName][deploymentName];
    const compute = this.#sdl.profiles?.compute?.[serviceDeployment.profile];
    const storages = castArray(compute?.resources.storage);

    storages.forEach((storage) => {
      const persistent = stringToBoolean(storage.attributes?.persistent as string | boolean || false);

      if (persistent && !service?.params?.storage?.[storage.name || ""]?.mount) {
        this.#errors.push({
          message: `Persistent storage "${storage.name || "default"}" requires a mount path in /services/${serviceName}/params/storage/${storage.name || "default"}/mount.`,
          instancePath: `/services/${serviceName}/params/storage/${storage.name || "default"}`,
          schemaPath: "#/properties/services/additionalProperties/properties/params/properties/storage/additionalProperties/properties/mount",
          keyword: "required",
          params: {
            missingProperty: "mount",
          },
        });
      }
    });
  }

  #validateGPU(serviceName: string, deploymentName: string) {
    const deployment = this.#sdl.deployment[serviceName];
    const compute = this.#sdl.profiles?.compute?.[deployment[deploymentName]?.profile];
    const gpu = compute?.resources.gpu;
    if (!gpu) return;

    const hasUnits = gpu.units !== undefined && gpu.units !== 0;
    const hasAttributes = typeof gpu.attributes !== "undefined";
    const hasVendor = hasAttributes && typeof gpu.attributes?.vendor !== "undefined";

    const profile = deployment[deploymentName]?.profile;
    const gpuPath = `/profiles/compute/${profile}/resources/gpu`;

    if (!hasUnits && hasAttributes) {
      this.#errors.push({
        message: "GPU must not have attributes if units is 0.",
        instancePath: `${gpuPath}/attributes`,
        schemaPath: "#/properties/profiles/properties/compute/additionalProperties/properties/resources/properties/gpu/properties/attributes",
        keyword: "additionalProperties",
        params: {
          additionalProperty: "attributes",
        },
      });
    }
    if (hasUnits && !hasAttributes) {
      this.#errors.push({
        message: "GPU must have attributes if units is not 0.",
        instancePath: gpuPath,
        schemaPath: "#/properties/profiles/properties/compute/additionalProperties/properties/resources/properties/gpu",
        keyword: "required",
        params: {
          missingProperty: "attributes",
        },
      });
    }
    if (hasUnits && !hasVendor) {
      this.#errors.push({
        message: "GPU must specify a vendor if units is not 0.",
        instancePath: `${gpuPath}/attributes`,
        schemaPath: "#/properties/profiles/properties/compute/additionalProperties/properties/resources/properties/gpu/properties/attributes/properties/vendor",
        keyword: "required",
        params: {
          missingProperty: "vendor",
        },
      });
    }

    // Note: the units==0 + interconnect/interconnect_group case is already rejected by
    // the schema-level rule `gpuAttributesRequireUnitsGt0` — it requires
    // units > 0 whenever any attribute (including interconnect / interconnect_group) is
    // present. So we don't need a duplicate semantic check here. The
    // Go parser (go/sdl/gpu.go) carries its own parse-time check because
    // YAML decoding has no schema layer above it.
  }

  // Mirror the Go-side validateInterconnect in go/sdl/v2.go. The TS
  // SDK enforces the same cross-field rules so tenants using
  // @akashnetwork/chain-sdk get the same fail-fast feedback as Go CLI
  // users (the chain doesn't validate SDL semantics). See
  // docs/sdl-interconnect-spec.md for the spec.
  //
  //   1. Any opt-in (implicit `[]` or explicit `{ group: <name> }`)
  //      requires the placement to require
  //      capabilities/gpu-interconnect="true".
  //   2. The reserved name `auto` may not be written explicitly under
  //      `interconnect: { group: ... }`. (Rule 2 is enforced at the
  //      schema level via the JSON-schema oneOf, but checked here too
  //      for a friendlier error message.)
  //   3. Within one placement, no mixing of implicit and explicit
  //      opt-in forms — either every service uses `[]` or every service
  //      uses a `{ group: ... }`.
  //   4. (Schema-level) `gpu.units == 0` with any interconnect opt-in
  //      is rejected.
  #validateInterconnect() {
    const profiles = this.#sdl.profiles?.compute;
    const placements = this.#sdl.profiles?.placement;
    if (!profiles) return;

    // Rule 2: tenants cannot write `group: auto` under the explicit
    // mapping form. The JSON schema's `oneOf` already rules this out at
    // the structural level when the value is bare, but we surface a
    // friendly error here.
    for (const [profileName, compute] of Object.entries(profiles)) {
      const gpu = compute?.resources?.gpu;
      if (!gpu || gpu.units === 0 || gpu.units === undefined) continue;
      const ic = gpu.attributes?.interconnect;
      if (ic && typeof ic === "object" && !Array.isArray(ic) && (ic as { group?: unknown }).group === INTERCONNECT_GROUP_AUTO) {
        this.#errors.push({
          message: `Compute profile "${profileName}" uses gpu.attributes.interconnect.group="${INTERCONNECT_GROUP_AUTO}", which is reserved (the parser auto-assigns it to "interconnect: []" resources). Pick a different group name.`,
          instancePath: `/profiles/compute/${profileName}/resources/gpu/attributes/interconnect/group`,
          schemaPath: "#/properties/profiles/properties/compute/additionalProperties/properties/resources/properties/gpu/properties/attributes/properties/interconnect",
          keyword: "const",
          params: { allowedValue: `non-${INTERCONNECT_GROUP_AUTO}` },
        });
      }
    }

    // Rules 1 + 3 are per-placement. Walk every (service, placement) and
    // capture which profile each service uses and whether it opted into
    // interconnect (implicit, explicit, or not at all).
    interface profileUsage {
      profileName: string;
      serviceName: string;
      group: string; // "" if non-interconnect, INTERCONNECT_GROUP_AUTO for implicit, else explicit name
    }
    const usagesByPlacement = new Map<string, profileUsage[]>();

    for (const [serviceName, perPlacement] of Object.entries(this.#sdl.deployment ?? {})) {
      for (const [placementName, svcdepl] of Object.entries(perPlacement ?? {})) {
        const compute = profiles[svcdepl.profile];
        if (!compute) continue;
        const gpu = compute.resources?.gpu;
        const usage: profileUsage = {
          profileName: svcdepl.profile,
          serviceName,
          group: "",
        };
        if (gpu && gpu.units !== 0 && gpu.units !== undefined) {
          usage.group = resolveInterconnectGroup(gpu.attributes?.interconnect);
        }

        const bucket = usagesByPlacement.get(placementName) ?? [];
        bucket.push(usage);
        usagesByPlacement.set(placementName, bucket);

        // Rule 1: any interconnect opt-in must be deployed under a
        // placement whose attributes require capabilities/gpu-interconnect=true.
        if (usage.group !== "") {
          const placement = placements?.[placementName];
          const attrs = (placement?.attributes ?? {}) as Record<string, unknown>;
          const capability = attrs["capabilities/gpu-interconnect"];
          const requiresInterconnect = capability === "true" || capability === true;
          if (!requiresInterconnect) {
            this.#errors.push({
              message: `Service "${serviceName}" uses interconnect profile "${svcdepl.profile}" under placement "${placementName}" but placement does not require capabilities/gpu-interconnect=true.`,
              instancePath: `/profiles/placement/${placementName}/attributes`,
              schemaPath: "#/properties/profiles/properties/placement/additionalProperties/properties/attributes",
              keyword: "required",
              params: { missingProperty: "capabilities/gpu-interconnect" },
            });
          }
        }
      }
    }

    // Rule 3: within one placement, no mixing of implicit (`auto`) and
    // explicit (named) opt-ins. Either form alone is fine; combining
    // them is the failure mode this rule catches.
    for (const [placementName, usages] of usagesByPlacement.entries()) {
      let firstImplicit: string | undefined;
      let firstExplicit: string | undefined;
      for (const u of usages) {
        if (u.group === "") continue;
        if (u.group === INTERCONNECT_GROUP_AUTO) {
          firstImplicit ??= u.profileName;
        } else {
          firstExplicit ??= u.profileName;
        }
      }
      if (firstImplicit !== undefined && firstExplicit !== undefined) {
        this.#errors.push({
          message: `Placement "${placementName}" mixes implicit "interconnect: []" (profile "${firstImplicit}") and explicit "interconnect: { group: ... }" (profile "${firstExplicit}"); use one form across the placement.`,
          instancePath: `/profiles/compute/${firstImplicit}/resources/gpu/attributes/interconnect`,
          schemaPath: "#/properties/profiles/properties/compute/additionalProperties/properties/resources/properties/gpu/properties/attributes/properties/interconnect",
          keyword: "oneOf",
          params: { passingSchemas: [] },
        });
      }
    }
  }

  // Mirrors Go's TEE rules from `buildGroups` (go/sdl/groupBuilder_v2.go:58-83,
  // groupBuilder_v2_1.go:59-84). Two checks, both on `deploymentName` (the
  // placement-group name):
  //   1. `validateTEEWithGPU` — the `cpu-gpu` type requires GPU resources on the
  //      resolved compute profile (`cpu`/absent need none).
  //   2. `errTEETypeMismatch` — a placementg diff group may carry only one tee type,
  //      since `generateManifest` projects it as a single `tee/type` requirement
  //      attribute. Detecting the conflict here (the validation layer) means
  //      standalone `validateSDL` callers catch it, and `generateManifest` can
  //      project unconditionally.
  // Unsupported tee values are rejected structurally by the input schema (`tee`
  // enum in validateSDLInput.ts) before this runs.
  #validateTEE(serviceName: string, deploymentName: string) {
    const tee = this.#sdl.services?.[serviceName]?.params?.tee;
    if (!tee) return;

    if (tee === "cpu-gpu") {
      const profile = this.#sdl.deployment[serviceName]?.[deploymentName]?.profile;
      const gpu = this.#sdl.profiles?.compute?.[profile]?.resources.gpu;
      const hasGpu = gpu?.units !== undefined && gpu.units !== 0;

      if (!hasGpu) {
        this.#errors.push({
          message: `Service "${serviceName}" tee type requires gpu resources.`,
          instancePath: `/services/${serviceName}/params/tee`,
          schemaPath: "#/properties/services/additionalProperties/properties/params/properties/tee",
          keyword: "required",
          params: {
            missingProperty: "gpu",
          },
        });
      }
    }

    const existing = this.#teeTypeByPlacement.get(deploymentName);
    if (existing === undefined) {
      this.#teeTypeByPlacement.set(deploymentName, tee);
    } else if (existing !== tee) {
      this.#errors.push({
        message: `conflicting tee types in placement group "${deploymentName}": "${existing}" and "${tee}"`,
        instancePath: `/services/${serviceName}/params/tee`,
        schemaPath: "#/properties/services/additionalProperties/properties/params/properties/tee",
        keyword: "tee",
        params: {},
      });
    }
  }

  #validateLeaseIP(serviceName: string) {
    this.#sdl.services?.[serviceName]?.expose?.forEach((expose, exposeIndex) => {
      const proto = expose.proto?.toUpperCase() || "TCP";

      expose.to?.forEach((to, toIndex) => {
        if (to.ip?.length) {
          const toPath = `/services/${serviceName}/expose/${exposeIndex}/to/${toIndex}`;

          if (!to.global) {
            this.#errors.push({
              message: `If an IP is declared, the directive must be declared as global.`,
              instancePath: `${toPath}/global`,
              schemaPath: "#/definitions/exposeToWithIpEnforcesGlobal/then/properties/global/const",
              keyword: "const",
              params: {
                allowedValue: true,
              },
            });
          }
          if (!this.#sdl.endpoints?.[to.ip]) {
            this.#errors.push({
              message: `Unknown endpoint "${to.ip}" for service "${serviceName}". Add it to the "endpoints" section.`,
              instancePath: `/endpoints/${to.ip}`,
              schemaPath: "#/properties/endpoints",
              keyword: "required",
              params: {
                missingProperty: to.ip,
              },
            });
          }

          this.#endpointsUsed.add(to.ip);

          const externalPort = expose.as ?? expose.port;
          const portKey = `${to.ip}-${externalPort}-${proto}`;
          const otherServiceName = this.#portsUsed.get(portKey);

          if (this.#portsUsed.has(portKey)) {
            this.#errors.push({
              message: `IP endpoint "${to.ip}" port ${externalPort} protocol ${proto} already in use by service "${otherServiceName}".`,
              instancePath: `${toPath}/ip`,
              schemaPath: "#/properties/services/additionalProperties/properties/expose/items/properties/to/items",
              keyword: "uniqueItems",
              params: {
                duplicate: portKey,
              },
            });
          }
          this.#portsUsed.set(portKey, serviceName);
        }
      });
    });
  }

  #validateEndpoints() {
    if (!this.#sdl.endpoints) return;

    Object.keys(this.#sdl.endpoints).forEach((endpoint) => {
      if (!this.#endpointsUsed.has(endpoint)) {
        this.#errors.push({
          message: `Endpoint "${endpoint}" declared but never used.`,
          instancePath: `/endpoints/${endpoint}`,
          schemaPath: "#/properties/endpoints",
          keyword: "additionalProperties",
          params: {
            additionalProperty: endpoint,
          },
        });
      }
    });
  }

  #validateNoVolumeReferences() {
    Object.entries(this.#sdl.services ?? {}).forEach(([serviceName, service]) => {
      Object.entries(service.params?.storage ?? {}).forEach(([storageName, storage]) => {
        if (!storage?.volume) return;

        this.#errors.push({
          message: `Service "${serviceName}" storage "${storageName}" sets volume; volume references require SDL version 2.2.`,
          instancePath: `/services/${serviceName}/params/storage/${storageName}/volume`,
          schemaPath: "#/properties/services/additionalProperties/properties/params/properties/storage/additionalProperties/properties/volume",
          keyword: "additionalProperties",
          params: {
            additionalProperty: "volume",
          },
        });
      });
    });
  }

  /**
   * Mirrors `validateContent` in go/sdl/v2_2.go. A volume is its own
   * deployment: an SDL either declares exactly one volume and no services, or
   * declares services that attach external volumes. Grammar the input schema
   * already enforces (vid labels, class, reclaim, retention, dseq) is not
   * repeated here.
   */
  #validateVolumes() {
    this.#validateVolumeDeclarations();
    this.#validateVolumeDeployments();
    this.#validateServiceVolumes();
    this.#validateSharedProfileVolumes();
    this.#validateUnusedVolumes();
  }

  #validateVolumeDeclarations() {
    const declared = declaredVolumeNames(this.#sdl);

    Object.entries(this.#sdl.volumes ?? {}).forEach(([name, volume]) => {
      if (this.#sdl.services?.[name]) {
        this.#errors.push({
          message: `Volume "${name}" collides with a service of the same name.`,
          instancePath: `/volumes/${name}`,
          schemaPath: "#/properties/volumes",
          keyword: "uniqueItems",
          params: {
            duplicate: name,
          },
        });
      }

      if (isExternalVolume(volume)) return;

      if (volumeSizeBytes(volume.size) === 0n) {
        this.#errors.push({
          message: `Volume "${name}" must declare a valid, non-zero size.`,
          instancePath: `/volumes/${name}/size`,
          schemaPath: "#/definitions/volumeDeclaration/properties/size",
          keyword: "exclusiveMinimum",
          params: {
            comparison: ">",
            limit: 0,
          },
        });
      }

      if (volume.adopt && volume["replica-of"]) {
        this.#errors.push({
          message: `Volume "${name}" sets both adopt and replica-of, which are mutually exclusive.`,
          instancePath: `/volumes/${name}`,
          schemaPath: "#/definitions/volumeDeclaration",
          keyword: "not",
          params: {},
        });
      }

      if (isBucketVolume(volume) && (volume["max-replicas"] || volume["replica-of"])) {
        this.#errors.push({
          message: `Volume "${name}" is a bucket, and a bucket has no replicas (max-replicas/replica-of).`,
          instancePath: `/volumes/${name}`,
          schemaPath: "#/definitions/volumeDeclaration/properties/access",
          keyword: "not",
          params: {},
        });
      }
    });

    if (declared.length && Object.keys(this.#sdl.services ?? {}).length) {
      this.#errors.push({
        message: "A volume deployment must not declare services; create the volume deployment separately and reference it via volumes.<name>.external.",
        instancePath: "/services",
        schemaPath: "#/properties/services",
        keyword: "not",
        params: {},
      });
    }

    if (declared.length > 1) {
      this.#errors.push({
        message: `Only one volume may be declared per SDL, since each volume is its own deployment; found ${declared.length}.`,
        instancePath: "/volumes",
        schemaPath: "#/properties/volumes",
        keyword: "maxProperties",
        params: {
          limit: 1,
        },
      });
    }
  }

  #validateVolumeDeployments() {
    Object.keys(this.#sdl.deployment).forEach((name) => {
      const volume = this.#sdl.volumes?.[name];

      if (volume && !isExternalVolume(volume)) {
        this.#validateVolumeDeployment(name, volume);
        return;
      }

      if (!this.#sdl.services?.[name]) {
        this.#errors.push({
          message: `Deployment "${name}" matches neither a service nor a declared volume.`,
          instancePath: `/deployment/${name}`,
          schemaPath: "#/properties/deployment",
          keyword: "additionalProperties",
          params: {
            additionalProperty: name,
          },
        });
      }
    });
  }

  #validateVolumeDeployment(name: string, volume: VolumeDeclaration) {
    const placements = Object.entries(this.#sdl.deployment[name]);

    this.#volumesUsed.add(name);

    if (placements.length !== 1) {
      this.#errors.push({
        message: `Volume "${name}" must be deployed to exactly one placement (${placements.length} given).`,
        instancePath: `/deployment/${name}`,
        schemaPath: "#/properties/deployment/additionalProperties",
        keyword: placements.length ? "maxProperties" : "minProperties",
        params: {
          limit: 1,
        },
      });
    }

    if (this.#sdl.profiles.compute?.[name]) {
      this.#errors.push({
        message: `Volume "${name}" collides with a compute profile of the same name.`,
        instancePath: `/profiles/compute/${name}`,
        schemaPath: "#/properties/profiles/properties/compute",
        keyword: "uniqueItems",
        params: {
          duplicate: name,
        },
      });
    }

    placements.forEach(([placementName, { profile, count }]) => {
      if (profile !== name) {
        this.#errors.push({
          message: `Volume "${name}" must be deployed with the profile "${name}", got "${profile}".`,
          instancePath: `/deployment/${name}/${placementName}/profile`,
          schemaPath: "#/properties/deployment/additionalProperties/additionalProperties/properties/profile",
          keyword: "const",
          params: {
            allowedValue: name,
          },
        });
      }

      this.#validatePlacementPricing(placementName, profile);

      if (count !== 1) {
        this.#errors.push({
          message: `Volume "${name}" deployment count must be 1, got ${count}.`,
          instancePath: `/deployment/${name}/${placementName}/count`,
          schemaPath: "#/properties/deployment/additionalProperties/additionalProperties/properties/count",
          keyword: "const",
          params: {
            allowedValue: 1,
          },
        });
      }
    });

    (["adopt", "replica-of"] as const).forEach((field) => {
      if (!volume[field] || this.#owner) return;

      this.#errors.push({
        message: `Volume "${name}" ${field} requires the owner; pass the signer as the owner option.`,
        instancePath: `/volumes/${name}/${field}`,
        schemaPath: "#/definitions/volumePolicyRef",
        keyword: "required",
        params: {
          missingProperty: "owner",
        },
      });
    });
  }

  #validateServiceVolumes() {
    const attachedBy = new Map<string, string>();
    const services = Object.entries(this.#sdl.services ?? {}).sort(([a], [b]) => compareStrings(a, b));

    services.forEach(([serviceName, service]) => {
      const placements = Object.entries(this.#sdl.deployment[serviceName] ?? {});
      const profiles = new Set(placements.map(([, { profile }]) => profile));

      Object.entries(service.params?.storage ?? {}).forEach(([storageName, storage]) => {
        const volumeName = storage?.volume;
        if (!volumeName) return;

        const storagePath = `/services/${serviceName}/params/storage/${storageName}`;
        const volume = this.#sdl.volumes?.[volumeName];

        if (!volume) {
          this.#errors.push({
            message: `Service "${serviceName}" references non-existing volume "${volumeName}".`,
            instancePath: "/volumes",
            schemaPath: "#/properties/volumes",
            keyword: "required",
            params: {
              missingProperty: volumeName,
            },
          });
          return;
        }

        if (!isExternalVolume(volume)) {
          this.#errors.push({
            message: `Service "${serviceName}" references volume "${volumeName}" which is declared in this SDL; a volume is its own deployment, create it first and reference it via volumes.${volumeName}.external.`,
            instancePath: `/volumes/${volumeName}`,
            schemaPath: "#/definitions/volumeExternalRef",
            keyword: "required",
            params: {
              missingProperty: "external",
            },
          });
          return;
        }

        profiles.forEach((profile) => {
          const storages = castArray(this.#sdl.profiles.compute?.[profile]?.resources.storage);
          if (!storages.some((entry) => (entry.name || "default") === storageName)) return;

          this.#errors.push({
            message: `Service "${serviceName}" storage "${storageName}" references volume "${volumeName}" and matches a compute profile storage entry; a volume reference excludes size, class and attributes.`,
            instancePath: `/profiles/compute/${profile}/resources/storage`,
            schemaPath: "#/properties/profiles/properties/compute/additionalProperties/properties/resources/properties/storage",
            keyword: "not",
            params: {},
          });
        });

        if (!storage.mount) {
          this.#errors.push({
            message: `Service "${serviceName}" storage "${storageName}" references volume "${volumeName}" and requires a mount.`,
            instancePath: storagePath,
            schemaPath: "#/properties/services/additionalProperties/properties/params/properties/storage/additionalProperties/properties/mount",
            keyword: "required",
            params: {
              missingProperty: "mount",
            },
          });
        }

        if (!volume.external.owner && !this.#owner) {
          this.#errors.push({
            message: `Volume "${volumeName}" owner is unknown; set volumes.${volumeName}.external.owner or pass the signer as the owner option.`,
            instancePath: `/volumes/${volumeName}/external`,
            schemaPath: "#/definitions/volumeExternalRef/properties/external",
            keyword: "required",
            params: {
              missingProperty: "owner",
            },
          });
        }

        const otherService = attachedBy.get(volumeName);
        if (otherService !== undefined) {
          this.#errors.push({
            message: otherService === serviceName
              ? `Service "${serviceName}" references volume "${volumeName}" more than once.`
              : `Volume "${volumeName}" is referenced by services "${otherService}" and "${serviceName}"; a volume attaches to a single service (RWO).`,
            instancePath: `${storagePath}/volume`,
            schemaPath: "#/properties/services/additionalProperties/properties/params/properties/storage/additionalProperties/properties/volume",
            keyword: "uniqueItems",
            params: {
              duplicate: volumeName,
            },
          });
        }

        attachedBy.set(volumeName, serviceName);
        this.#volumesUsed.add(volumeName);
      });

      if (!serviceVolumeNames(service).length) return;

      placements.forEach(([placementName, { count }]) => {
        if (count === 1) return;

        this.#errors.push({
          message: `Service "${serviceName}" attaches volumes with count ${count}; volumes attach once (RWO), so count must be 1.`,
          instancePath: `/deployment/${serviceName}/${placementName}/count`,
          schemaPath: "#/properties/deployment/additionalProperties/additionalProperties/properties/count",
          keyword: "const",
          params: {
            allowedValue: 1,
          },
        });
      });
    });
  }

  /**
   * Services sharing a compute profile share one resource unit, and the
   * provider requires every service of a unit to carry exactly the unit's
   * volume references (`buildGroups` in go/sdl/groupBuilder_v2_2.go).
   */
  #validateSharedProfileVolumes() {
    const unitVolumes = new Map<string, string>();
    const services = this.#sdl.services ?? {};

    Object.keys(services).sort(compareStrings).forEach((serviceName) => {
      const volumes = JSON.stringify(serviceVolumeNames(services[serviceName]).sort(compareStrings));

      Object.entries(this.#sdl.deployment[serviceName] ?? {}).forEach(([placementName, { profile }]) => {
        const unit = JSON.stringify([placementName, profile]);
        const boundVolumes = unitVolumes.get(unit);

        if (boundVolumes === undefined) {
          unitVolumes.set(unit, volumes);
          return;
        }

        if (boundVolumes === volumes) return;

        this.#errors.push({
          message: `Service "${serviceName}" shares compute profile "${profile}" in placement "${placementName}" but not its volume references; give volume-mounting services a dedicated profile.`,
          instancePath: `/deployment/${serviceName}/${placementName}/profile`,
          schemaPath: "#/properties/deployment/additionalProperties/additionalProperties/properties/profile",
          keyword: "uniqueItems",
          params: {
            duplicate: profile,
          },
        });
      });
    });
  }

  #validateUnusedVolumes() {
    Object.keys(this.#sdl.volumes ?? {}).forEach((name) => {
      if (this.#volumesUsed.has(name)) return;

      this.#errors.push({
        message: `Volume "${name}" declared but never used.`,
        instancePath: `/volumes/${name}`,
        schemaPath: "#/properties/volumes",
        keyword: "additionalProperties",
        params: {
          additionalProperty: name,
        },
      });
    });
  }
}

function volumeSizeBytes(size: string): bigint {
  try {
    return parseStorageBytes(size);
  } catch {
    return 0n;
  }
}
