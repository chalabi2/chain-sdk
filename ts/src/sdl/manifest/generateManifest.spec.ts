import fs from "node:fs";
import path from "node:path";

import { describe, expect, it } from "vitest";

import { VolumePolicy, VolumePolicy_Access, VolumePolicy_ReclaimPolicy } from "../../generated/protos/index.akash.v1.ts";
import { GroupSpec } from "../../generated/protos/index.akash.v1beta4.ts";
import { Group } from "../../generated/protos/index.provider.akash.v2beta3.ts";
import { yaml } from "../../utils/yaml.ts";
import type { SDLInput } from "../validateSDL/validateSDL.ts";
import type { GenerateManifestResult } from "./generateManifest.ts";
import { generateManifest } from "./generateManifest.ts";
import type { SDLCompute, SDLService } from "./manifestUtils.ts";

describe(generateManifest.name, () => {
  describe("basic manifest generation", () => {
    it("generates manifest with correct structure", () => {
      const { result } = setup();

      expect(result.groups).toHaveLength(1);
      expect(result.groups[0].name).toBe("dcloud");
      expect(result.groups[0].services).toHaveLength(1);
      expect(result.groups[0].services[0].name).toBe("web");
      expect(result.groups[0].services[0].image).toBe("nginx");
    });

    it("generates groupSpecs with correct structure", () => {
      const { result } = setup();

      expect(result.groupSpecs).toHaveLength(1);
      expect(result.groupSpecs[0].name).toBe("dcloud");
      expect(result.groupSpecs[0].resources).toHaveLength(1);
      expect(result.groupSpecs[0].resources[0].count).toBe(1);
      expect(result.groupSpecs[0].resources[0].price).toEqual({
        denom: "uakt",
        amount: "1000",
      });
    });

    it("parses CPU units correctly", () => {
      const sdl = createBasicSdl({ cpu: 0.5 });
      const { result } = setup({ sdl });

      const cpuVal = result.groups[0].services[0].resources?.cpu?.units?.val;
      expect(cpuVal).toBe(500n);
    });

    it("parses CPU units from string format", () => {
      const sdl = createBasicSdl({ cpu: "100m" });
      const { result } = setup({ sdl });

      const cpuVal = result.groups[0].services[0].resources?.cpu?.units?.val;
      expect(cpuVal).toBe(100n);
    });

    it("parses memory size correctly", () => {
      const sdl = createBasicSdl({ memory: "512Mi" });
      const { result } = setup({ sdl });

      const memVal = result.groups[0].services[0].resources?.memory?.quantity?.val;
      expect(memVal).toBe(536870912n);
    });

    it("parses storage correctly", () => {
      const sdl = createBasicSdl({ storage: [{ size: "1Gi" }] });
      const { result } = setup({ sdl });

      const storage = result.groups[0].services[0].resources?.storage;
      expect(storage).toHaveLength(1);
      expect(storage?.[0].name).toBe("default");
      const storageVal = storage?.[0].quantity?.val;
      expect(storageVal).toBe(1073741824n);
    });

    it("parses storage params in ASC order", () => {
      const sdl: SDLInput = yaml`
        version: "2.0"
        services:
          web:
            image: nginx
            params:
              storage:
                data:
                  mount: /mnt/data
                  readOnly: true
                cache:
                  mount: /mnt/cache
        profiles:
          compute:
            web:
              resources:
                cpu:
                  units: 0.5
                memory:
                  size: 512Mi
                storage:
                  - name: data
                    size: 1Gi
                  - name: cache
                    size: 512Mi
          placement:
            datacenter:
              pricing:
                web:
                  denom: uakt
                  amount: 200
        deployment:
          web:
            datacenter:
              profile: web
              count: 1
      `;
      const { result } = setup({ sdl });
      expect(result.groups[0].services[0].params?.storage).toEqual([
        expect.objectContaining({ name: "cache" }),
        expect.objectContaining({ name: "data" }),
      ]);
    });
  });

  describe("service features", () => {
    it("includes credentials when provided", () => {
      const credentials = {
        host: "registry.example.com",
        username: "user",
        password: "password123",
      };
      const sdl = createBasicSdl({ credentials });
      const { result } = setup({ sdl });

      expect(result.groups[0].services[0].credentials).toMatchObject({
        host: "registry.example.com",
        username: "user",
        password: "password123",
        email: "",
      });
    });

    it("excludes credentials when not provided", () => {
      const sdl = createBasicSdl();
      const { result } = setup({ sdl });

      expect(result.groups[0].services[0].credentials).toBeUndefined();
    });

    it("includes environment variables", () => {
      const env = ["ENV1=value1", "ENV2=value2"];
      const sdl = createBasicSdl({ env });
      const { result } = setup({ sdl });

      expect(result.groups[0].services[0].env).toEqual(env);
    });

    it("includes command and args", () => {
      const command = ["/bin/sh"];
      const args = ["-c", "echo hello"];
      const sdl = createBasicSdl({ command, args });
      const { result } = setup({ sdl });

      expect(result.groups[0].services[0].command).toEqual(command);
      expect(result.groups[0].services[0].args).toEqual(args);
    });
  });

  describe("GPU configuration", () => {
    it("handles GPU with units=0", () => {
      const sdl = createBasicSdl({ gpu: { units: 0 } });
      const { result } = setup({ sdl });

      const gpuVal = result.groups[0].services[0].resources?.gpu?.units?.val;
      expect(gpuVal).toBe(0n);
    });

    it("handles GPU with units and vendor attributes", () => {
      const sdl = createBasicSdl({
        gpu: {
          units: 1,
          attributes: {
            vendor: {
              nvidia: [{ model: "rtxa6000" }],
            },
          },
        },
      });
      const { result } = setup({ sdl });

      const gpuVal = result.groups[0].services[0].resources?.gpu?.units?.val;
      expect(gpuVal).toBe(1n);
      expect(result.groups[0].services[0].resources?.gpu?.attributes).toContainEqual({
        key: "vendor/nvidia/model/rtxa6000",
        value: "true",
      });
    });

    it("handles GPU with ram and interface", () => {
      const sdl = createBasicSdl({
        gpu: {
          units: 1,
          attributes: {
            vendor: {
              nvidia: [{ model: "rtxa6000", ram: "48Gi", interface: "pcie" }],
            },
          },
        },
      });
      const { result } = setup({ sdl });

      expect(result.groups[0].services[0].resources?.gpu?.attributes).toContainEqual({
        key: "vendor/nvidia/model/rtxa6000/ram/48Gi/interface/pcie",
        value: "true",
      });
    });
  });

  describe("TEE configuration", () => {
    it("projects tee cpu into service params with attestation true", () => {
      const sdl = createBasicSdl();
      sdl.services!.web.params = { tee: "cpu" };
      const { result } = setup({ sdl });

      expect(result.groups[0].services[0].params?.tee).toEqual({ type: "cpu", attestation: true });
    });

    it("projects tee cpu-gpu into service params with attestation true", () => {
      const sdl = createBasicSdl({
        gpu: { units: 1, attributes: { vendor: { nvidia: [{ model: "a100" }] } } },
      });
      sdl.services!.web.params = { tee: "cpu-gpu" };
      const { result } = setup({ sdl });

      expect(result.groups[0].services[0].params?.tee).toEqual({ type: "cpu-gpu", attestation: true });
    });

    it("omits tee from params when not set", () => {
      const sdl = createBasicSdl();
      sdl.services!.web.params = { permissions: { read: ["logs"] } };
      const { result } = setup({ sdl });

      expect(result.groups[0].services[0].params?.tee).toBeUndefined();
    });

    it("projects tee cpu as a sorted tee/type group-spec requirement attribute", () => {
      const sdl = createBasicSdl({ placementAttributes: { region: "us-west", zone: "z1" } });
      sdl.services!.web.params = { tee: "cpu" };
      const { result } = setup({ sdl });

      const attributes = result.groupSpecs[0].requirements?.attributes;
      expect(attributes).toContainEqual({ key: "tee/type", value: "cpu" });
      // attributes stay sorted by key (canonical on-chain ordering); tee/type
      // sorts between region and zone.
      expect(attributes?.map((a) => a.key)).toEqual(["region", "tee/type", "zone"]);
    });

    it("projects tee cpu-gpu as a tee/type group-spec requirement attribute", () => {
      const sdl = createBasicSdl({
        gpu: { units: 1, attributes: { vendor: { nvidia: [{ model: "a100" }] } } },
      });
      sdl.services!.web.params = { tee: "cpu-gpu" };
      const { result } = setup({ sdl });

      expect(result.groupSpecs[0].requirements?.attributes).toContainEqual({ key: "tee/type", value: "cpu-gpu" });
    });

    it("adds no tee/type attribute when tee is not set", () => {
      const sdl = createBasicSdl({ placementAttributes: { region: "us-west" } });
      const { result } = setup({ sdl });

      const attributes = result.groupSpecs[0].requirements?.attributes ?? [];
      expect(attributes.some((a) => a.key === "tee/type")).toBe(false);
    });

    it("rejects conflicting tee types within the same placement group", () => {
      const sdl: SDLInput = yaml`
        version: "2.1"
        services:
          alpha:
            image: nginx
            expose:
              - port: 80
                as: 80
                to:
                  - global: true
            params:
              tee: cpu
          beta:
            image: nginx
            expose:
              - port: 81
                as: 81
                to:
                  - global: true
            params:
              tee: cpu-gpu
        profiles:
          compute:
            alpha:
              resources:
                cpu:
                  units: 0.5
                memory:
                  size: 512Mi
                storage:
                  size: 512Mi
            beta:
              resources:
                cpu:
                  units: 0.5
                memory:
                  size: 512Mi
                storage:
                  size: 512Mi
                gpu:
                  units: 1
                  attributes:
                    vendor:
                      nvidia:
                        - model: a100
          placement:
            dcloud:
              pricing:
                alpha:
                  denom: uakt
                  amount: 1000
                beta:
                  denom: uakt
                  amount: 1000
        deployment:
          alpha:
            dcloud:
              profile: alpha
              count: 1
          beta:
            dcloud:
              profile: beta
              count: 1
      `;

      const result = generateManifest(sdl);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.value).toContainEqual(expect.objectContaining({
          instancePath: "/services/beta/params/tee",
          keyword: "tee",
          message: expect.stringContaining("conflicting tee types"),
        }));
      }
    });

    it("emits a single tee/type attribute for two same-tee services sharing a group", () => {
      const sdl: SDLInput = yaml`
        version: "2.1"
        services:
          alpha:
            image: nginx
            expose:
              - port: 80
                as: 80
                to:
                  - global: true
            params:
              tee: cpu
          beta:
            image: nginx
            expose:
              - port: 81
                as: 81
                to:
                  - global: true
            params:
              tee: cpu
        profiles:
          compute:
            alpha:
              resources:
                cpu:
                  units: 0.5
                memory:
                  size: 512Mi
                storage:
                  size: 512Mi
            beta:
              resources:
                cpu:
                  units: 0.5
                memory:
                  size: 512Mi
                storage:
                  size: 512Mi
          placement:
            dcloud:
              pricing:
                alpha:
                  denom: uakt
                  amount: 1000
                beta:
                  denom: uakt
                  amount: 1000
        deployment:
          alpha:
            dcloud:
              profile: alpha
              count: 1
          beta:
            dcloud:
              profile: beta
              count: 1
      `;
      const { result } = setup({ sdl });

      const teeAttributes = (result.groupSpecs[0].requirements?.attributes ?? []).filter((a) => a.key === "tee/type");
      expect(teeAttributes).toEqual([{ key: "tee/type", value: "cpu" }]);
    });

    // Cross-check the projected group-spec attributes against the Go parser's
    // golden output for the shared TEE parity fixtures (generated by
    // `make generate-sdl-fixtures`). This locks bit-for-bit parity with
    // go/sdl/groupBuilder_v2_1.go's `tee/type` projection.
    describe("Go parser parity (group-spec attributes)", () => {
      const TESTDATA_ROOT = path.join(__dirname, "../../../../testdata/sdl");

      it.each([
        ["tee-cpu", "cpu"],
        ["tee-cpu-gpu", "cpu-gpu"],
      ])("matches Go group-spec requirement attributes for the %s fixture", (fixture, teeType) => {
        const input = fs.readFileSync(path.join(TESTDATA_ROOT, "input/v2.1", fixture, "input.yaml"), "utf8");
        const sdl = yaml.raw<SDLInput>(input);
        const { result } = setup({ sdl });

        const golden = JSON.parse(
          fs.readFileSync(path.join(TESTDATA_ROOT, "output-fixtures/v2.1", fixture, "group-specs.json"), "utf8"),
        );

        expect(result.groupSpecs).toHaveLength(golden.length);
        expect(result.groupSpecs[0].requirements?.attributes).toEqual(golden[0].requirements.attributes);
        expect(result.groupSpecs[0].requirements?.attributes).toContainEqual({ key: "tee/type", value: teeType });
      });
    });
  });

  describe("storage configuration", () => {
    it("handles array of storage volumes", () => {
      const sdl = createBasicSdl({
        storage: [
          { name: "data", size: "1Gi" },
          { name: "cache", size: "512Mi" },
        ],
      });
      const { result } = setup({ sdl });

      const storage = result.groups[0].services[0].resources?.storage;
      expect(storage).toHaveLength(2);
      expect(storage?.[0].name).toBe("data");
      expect(storage?.[1].name).toBe("cache");
    });

    it("handles persistent storage attributes", () => {
      const sdl: SDLInput = yaml`
        version: "2.0"
        services:
          web:
            image: nginx
            expose:
              - port: 80
                as: 80
                to:
                  - global: true
            params:
              storage:
                data:
                  mount: /mnt/data
        profiles:
          compute:
            web:
              resources:
                cpu:
                  units: 0.5
                memory:
                  size: 512Mi
                storage:
                  - name: data
                    size: 1Gi
                    attributes:
                      persistent: true
                      class: beta2
          placement:
            dcloud:
              pricing:
                web:
                  denom: uakt
                  amount: 1000
        deployment:
          web:
            dcloud:
              profile: web
              count: 1
      `;
      const { result } = setup({ sdl });

      const storage = result.groups[0].services[0].resources?.storage;
      expect(storage?.[0].attributes).toContainEqual({ key: "persistent", value: "true" });
      expect(storage?.[0].attributes).toContainEqual({ key: "class", value: "beta2" });
    });
  });

  describe("placement and deployment", () => {
    it("includes placement attributes", () => {
      const sdl = createBasicSdl({
        placementAttributes: { region: "us-west", datacenter: "equinix" },
      });
      const { result } = setup({ sdl });

      expect(result.groupSpecs[0].requirements?.attributes).toContainEqual({
        key: "datacenter",
        value: "equinix",
      });
      expect(result.groupSpecs[0].requirements?.attributes).toContainEqual({
        key: "region",
        value: "us-west",
      });
    });

    it("includes signedBy requirements", () => {
      const sdl = createBasicSdl({
        signedBy: {
          anyOf: ["akash1abc", "akash1def"],
          allOf: ["akash1xyz"],
        },
      });
      const { result } = setup({ sdl });

      expect(result.groupSpecs[0].requirements?.signedBy?.anyOf).toEqual(["akash1abc", "akash1def"]);
      expect(result.groupSpecs[0].requirements?.signedBy?.allOf).toEqual(["akash1xyz"]);
    });

    it("sorts placements alphabetically", () => {
      const sdl: SDLInput = yaml`
        version: "2.0"
        services:
          web:
            image: nginx
            expose:
              - port: 80
                as: 80
                to:
                  - global: true
        profiles:
          compute:
            web:
              resources:
                cpu:
                  units: 0.5
                memory:
                  size: 512Mi
                storage:
                  size: 512Mi
          placement:
            zebra:
              pricing:
                web:
                  denom: uakt
                  amount: 1000
            alpha:
              pricing:
                web:
                  denom: uakt
                  amount: 1000
        deployment:
          web:
            zebra:
              profile: web
              count: 1
            alpha:
              profile: web
              count: 1
      `;
      const { result } = setup({ sdl });

      expect(result.groups.map((g) => g.name)).toEqual(["alpha", "zebra"]);
      expect(result.groupSpecs.map((g) => g.name)).toEqual(["alpha", "zebra"]);
    });

    it("sorts services alphabetically within placement", () => {
      const sdl: SDLInput = yaml`
        version: "2.0"
        services:
          zebra:
            image: nginx
            expose:
              - port: 80
                as: 80
                to:
                  - global: true
          alpha:
            image: nginx
            expose:
              - port: 81
                as: 81
                to:
                  - global: true
        profiles:
          compute:
            zebra:
              resources:
                cpu:
                  units: 0.5
                memory:
                  size: 512Mi
                storage:
                  size: 512Mi
            alpha:
              resources:
                cpu:
                  units: 0.5
                memory:
                  size: 512Mi
                storage:
                  size: 512Mi
          placement:
            dcloud:
              pricing:
                zebra:
                  denom: uakt
                  amount: 1000
                alpha:
                  denom: uakt
                  amount: 1000
        deployment:
          zebra:
            dcloud:
              profile: zebra
              count: 1
          alpha:
            dcloud:
              profile: alpha
              count: 1
      `;
      const { result } = setup({ sdl });

      expect(result.groups[0].services.map((s) => s.name)).toEqual(["alpha", "zebra"]);
    });
  });

  describe("endpoints and expose", () => {
    it("handles global endpoint", () => {
      const { result } = setup();

      const expose = result.groups[0].services[0].expose;
      expect(expose).toHaveLength(1);
      expect(expose[0].port).toBe(80);
      expect(expose[0].externalPort).toBe(80);
      expect(expose[0].global).toBe(true);
      expect(expose[0].proto).toBe("TCP");
    });

    it("handles UDP protocol", () => {
      const sdl = createBasicSdl({
        expose: [{ port: 12345, as: 12345, proto: "udp", to: [{ global: true }] }],
      });
      const { result } = setup({ sdl });

      expect(result.groups[0].services[0].expose[0].proto).toBe("UDP");
    });

    it("handles HTTP options", () => {
      const sdl = createBasicSdl({
        expose: [
          {
            port: 80,
            as: 80,
            to: [{ global: true }],
            http_options: {
              max_body_size: 104857600,
              read_timeout: 50,
              send_timeout: 100,
              next_tries: 24,
              next_timeout: 48,
              next_cases: ["500"],
            },
          },
        ],
      });
      const { result } = setup({ sdl });

      const httpOptions = result.groups[0].services[0].expose[0].httpOptions;
      expect(httpOptions?.maxBodySize).toBe(104857600);
      expect(httpOptions?.readTimeout).toBe(50);
      expect(httpOptions?.sendTimeout).toBe(100);
      expect(httpOptions?.nextTries).toBe(24);
      expect(httpOptions?.nextTimeout).toBe(48);
      expect(httpOptions?.nextCases).toEqual(["500"]);
    });

    it("handles HTTP proxy options", () => {
      const sdl = createBasicSdl({
        expose: [
          {
            port: 80,
            as: 80,
            to: [{ global: true }],
            http_options: {
              proxy: {
                buffering_disable: true,
                buffer_size: 4096,
                buffers_number: 8,
                buffers_size: 4096,
                busy_buffers_size: 8192,
                connect_timeout: 5000,
              },
            },
          },
        ],
      });
      const { result } = setup({ sdl });

      const proxy = result.groups[0].services[0].expose[0].httpOptions?.proxy;
      expect(proxy?.bufferingDisable).toBe(true);
      expect(proxy?.bufferSize).toBe(4096);
      expect(proxy?.buffersNumber).toBe(8);
      expect(proxy?.buffersSize).toBe(4096);
      expect(proxy?.busyBuffersSize).toBe(8192);
      expect(proxy?.connectTimeout).toBe(5000);
    });

    it("omits HTTP proxy options when the proxy block is empty", () => {
      const sdl = createBasicSdl({
        expose: [
          {
            port: 80,
            as: 80,
            to: [{ global: true }],
            http_options: { proxy: {} },
          },
        ],
      });
      const { result } = setup({ sdl });

      expect(result.groups[0].services[0].expose[0].httpOptions?.proxy).toBeUndefined();
    });

    it("rejects HTTP proxy options with buffers_number but no buffers_size", () => {
      const sdl = createBasicSdl({
        expose: [
          {
            port: 80,
            as: 80,
            to: [{ global: true }],
            http_options: { proxy: { buffers_number: 8 } },
          },
        ],
      });

      // The paired-field rule is enforced by the schema, so the SDL is rejected at
      // validation time rather than throwing later when the manifest is built.
      const result = generateManifest(sdl);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.value).toContainEqual(expect.objectContaining({ keyword: "dependencies" }));
      }
    });

    it("handles accept hosts", () => {
      const sdl = createBasicSdl({
        expose: [{ port: 80, as: 80, to: [{ global: true }], accept: ["example.com", "api.example.com"] }],
      });
      const { result } = setup({ sdl });

      expect(result.groups[0].services[0].expose[0].hosts).toEqual(["example.com", "api.example.com"]);
    });

    it("handles IP lease endpoints", () => {
      const sdl: SDLInput = yaml`
        version: "2.0"
        endpoints:
          myip:
            kind: ip
        services:
          web:
            image: nginx
            expose:
              - port: 80
                as: 80
                to:
                  - global: true
                    ip: myip
        profiles:
          compute:
            web:
              resources:
                cpu:
                  units: 0.5
                memory:
                  size: 512Mi
                storage:
                  size: 512Mi
          placement:
            dcloud:
              pricing:
                web:
                  denom: uakt
                  amount: 1000
        deployment:
          web:
            dcloud:
              profile: web
              count: 1
      `;
      const { result } = setup({ sdl });

      expect(result.groups[0].services[0].expose[0].ip).toBe("myip");
      expect(result.groups[0].services[0].expose[0].endpointSequenceNumber).toBe(1);

      // Check endpoint in resources
      const endpoints = result.groups[0].services[0].resources?.endpoints;
      expect(endpoints).toContainEqual(expect.objectContaining({ kind: 2, sequenceNumber: 1 }));
    });

    it("assigns sequence numbers to global endpoints in sorted order", async () => {
      const sdlContent = readFileSync("../fixtures/shared-ip.yml");
      const sdl = yaml.template<SDLInput>(sdlContent);
      const { result } = setup({ sdl });

      const sequenceNumbers = result.groups
        .flatMap((g) => g.services)
        .flatMap((s) => s.expose ?? [])
        .map((e) => e.endpointSequenceNumber);

      expect(sequenceNumbers).toEqual([2, 2]);
    });
  });

  describe("complex scenarios", () => {
    it("handles multi-service SDL", () => {
      const sdl: SDLInput = yaml`
        version: "2.0"
        services:
          web:
            image: wordpress
            expose:
              - port: 80
                as: 80
                to:
                  - global: true
          db:
            image: mysql
            expose:
              - port: 3306
                to:
                  - service: web
        profiles:
          compute:
            web:
              resources:
                cpu:
                  units: 1
                memory:
                  size: 1Gi
                storage:
                  size: 2Gi
            db:
              resources:
                cpu:
                  units: 0.5
                memory:
                  size: 512Mi
                storage:
                  size: 1Gi
          placement:
            dcloud:
              pricing:
                web:
                  denom: uakt
                  amount: 2000
                db:
                  denom: uakt
                  amount: 1000
        deployment:
          web:
            dcloud:
              profile: web
              count: 1
          db:
            dcloud:
              profile: db
              count: 1
      `;
      const { result } = setup({ sdl });

      expect(result.groups).toHaveLength(1);
      expect(result.groups[0].services).toHaveLength(2);
      expect(result.groups[0].services.map((s) => s.name)).toEqual(["db", "web"]);

      // GroupSpecs have 2 resource units (different profiles)
      expect(result.groupSpecs[0].resources).toHaveLength(2);
    });

    it("handles service-to-service expose", () => {
      const sdl: SDLInput = yaml`
        version: "2.0"
        services:
          api:
            image: api:latest
            expose:
              - port: 3000
                to:
                  - service: web
          web:
            image: web:latest
            expose:
              - port: 80
                as: 80
                to:
                  - global: true
        profiles:
          compute:
            api:
              resources:
                cpu:
                  units: 0.5
                memory:
                  size: 512Mi
                storage:
                  size: 512Mi
            web:
              resources:
                cpu:
                  units: 0.5
                memory:
                  size: 512Mi
                storage:
                  size: 512Mi
          placement:
            dcloud:
              pricing:
                api:
                  denom: uakt
                  amount: 1000
                web:
                  denom: uakt
                  amount: 1000
        deployment:
          api:
            dcloud:
              profile: api
              count: 1
          web:
            dcloud:
              profile: web
              count: 1
      `;
      const { result } = setup({ sdl });

      const apiService = result.groups[0].services.find((s) => s.name === "api");
      expect(apiService?.expose[0].service).toBe("web");
      expect(apiService?.expose[0].global).toBe(false);
    });
  });

  describe("fixture file tests", () => {
    describe("GPU basic", () => {
      const sdlContent = readFileSync("../fixtures/gpu_basic.sdl.yml");

      it("generates manifest (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroups(result.groups)).toMatchSnapshot("GPU basic manifest");
      });

      it("generates groupSpecs (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroupSpecs(result.groupSpecs)).toMatchSnapshot("GPU basic groupSpecs");
      });
    });

    describe("GPU basic with RAM", () => {
      const sdlContent = readFileSync("../fixtures/gpu_basic_ram.sdl.yml");

      it("generates manifest (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroups(result.groups)).toMatchSnapshot("GPU basic RAM manifest");
      });

      it("generates groupSpecs (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroupSpecs(result.groupSpecs)).toMatchSnapshot("GPU basic RAM groupSpecs");
      });
    });

    describe("GPU basic with RAM and interface", () => {
      const sdlContent = readFileSync("../fixtures/gpu_basic_ram_interface.sdl.yml");

      it("generates manifest (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroups(result.groups)).toMatchSnapshot("GPU basic RAM interface manifest");
      });

      it("generates groupSpecs (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroupSpecs(result.groupSpecs)).toMatchSnapshot("GPU basic RAM interface groupSpecs");
      });
    });

    describe("WordPress multi-service", () => {
      const sdlContent = readFileSync("../fixtures/wordpress.sdl.yml");

      it("generates manifest (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroups(result.groups)).toMatchSnapshot("WordPress manifest");
      });

      it("generates groupSpecs (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroupSpecs(result.groupSpecs)).toMatchSnapshot("WordPress groupSpecs");
      });
    });

    describe("Persistent storage", () => {
      const sdlContent = readFileSync("../fixtures/persistent_storage_valid.sdl.yml");

      it("generates manifest (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroups(result.groups)).toMatchSnapshot("Persistent storage manifest");
      });

      it("generates groupSpecs (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroupSpecs(result.groupSpecs)).toMatchSnapshot("Persistent storage groupSpecs");
      });
    });

    describe("IP lease", () => {
      const sdlContent = readFileSync("../fixtures/ip_lease_valid.sdl.yml");

      it("generates manifest (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroups(result.groups)).toMatchSnapshot("IP lease manifest");
      });

      it("generates groupSpecs (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroupSpecs(result.groupSpecs)).toMatchSnapshot("IP lease groupSpecs");
      });
    });

    describe("GPU without model", () => {
      const sdlContent = readFileSync("../fixtures/gpu_basic_no_model.sdl.yml");

      it("generates manifest (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroups(result.groups)).toMatchSnapshot("GPU no model manifest");
      });

      it("generates groupSpecs (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroupSpecs(result.groupSpecs)).toMatchSnapshot("GPU no model groupSpecs");
      });
    });

    describe("GPU without GPU (units=0)", () => {
      const sdlContent = readFileSync("../fixtures/gpu_no_gpu_valid.sdl.yml");

      it("generates manifest (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroups(result.groups)).toMatchSnapshot("GPU zero units manifest");
      });

      it("generates groupSpecs (snapshot)", () => {
        const sdl = yaml.template<SDLInput>(sdlContent);
        const { result } = setup({ sdl });

        expect(stringifyGroupSpecs(result.groupSpecs)).toMatchSnapshot("GPU zero units groupSpecs");
      });
    });
  });

  describe("reclamation", () => {
    it.each([
      ["24h", "86400"],
      ["30m", "1800"],
      ["720h", "2592000"],
      ["8760h", "31536000"],
    ])("surfaces min_window %j as a proto Duration (%s seconds)", (minWindow, seconds) => {
      const { result } = setup({ sdl: createBasicSdl({ reclamation: { min_window: minWindow } }) });
      expect(result.reclamation?.minWindow?.seconds.toString()).toBe(seconds);
      expect(result.reclamation?.minWindow?.nanos).toBe(0);
    });

    it("leaves reclamation undefined when no block is present", () => {
      const { result } = setup();
      expect(result.reclamation).toBeUndefined();
    });

    // Compound ("1h30m"), fractional ("1.5h") and sub-second ("500ms") forms are
    // accepted by Go's `time.ParseDuration` but rejected by the stricter SDL
    // schema pattern (`^[1-9][0-9]*(s|m|h)$`).
    it.each(["abc", "0s", "-1h", "100", "1h30m", "1.5h", "500ms"])("rejects an invalid min_window %j", (minWindow) => {
      const result = generateManifest(createBasicSdl({ reclamation: { min_window: minWindow } }));
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.value).toContainEqual(expect.objectContaining({
          instancePath: "/reclamation/min_window",
        }));
      }
    });

    it("exposes reclamation as a lazy, memoized getter", () => {
      const { result } = setup({ sdl: createBasicSdl({ reclamation: { min_window: "24h" } }) });
      // It's a getter (computed on access, not eagerly)...
      expect(Object.getOwnPropertyDescriptor(result, "reclamation")?.get).toBeTypeOf("function");
      // ...and memoized: repeated reads return the same instance.
      expect(result.reclamation).toBe(result.reclamation);
    });
  });

  describe("volumes (SDL 2.2)", () => {
    const owner = "akash1qufa3xzrf34qvwyejkhvnc4psf49fftaw0asmh";

    it("compiles a declared volume into a storage-only group with an empty manifest group", () => {
      const sdl = createVolumeSdl({ lifecycle: { reclaim: "delete", retention: "2h" }, "max-replicas": 3 });
      const { result } = setup({ sdl });

      expect(result.groupSpecs).toEqual([
        GroupSpec.fromPartial({
          name: "us-west",
          requirements: {
            attributes: [
              { key: "capabilities/storage/volumes", value: "true" },
              { key: "region", value: "us-west" },
            ],
            signedBy: {},
          },
          resources: [
            {
              resource: {
                id: 1,
                cpu: { units: { val: 0n } },
                memory: { quantity: { val: 0n } },
                storage: [
                  {
                    name: "myapp-pgdata",
                    quantity: { val: 10737418240n },
                    attributes: [
                      { key: "class", value: "beta2" },
                      { key: "persistent", value: "true" },
                    ],
                  },
                ],
                gpu: { units: { val: 0n } },
                endpoints: [],
              },
              count: 1,
              price: { denom: "uakt", amount: "150" },
            },
          ],
          volume: {
            vid: "myapp-pgdata",
            reclaim: VolumePolicy_ReclaimPolicy.delete,
            retention: { seconds: 7200n, nanos: 0 },
            maxAttachments: 1,
            maxReplicas: 3,
          },
        }),
      ]);
      expect(result.groups).toEqual([Group.fromPartial({ name: "us-west", services: [] })]);
    });

    it("defaults the storage class, the reclaim policy and the retention", () => {
      const { result } = setup({ sdl: createVolumeSdl({ class: undefined }) });
      const [group] = result.groupSpecs;

      expect(group.resources[0].resource?.storage[0].attributes).toEqual([
        { key: "class", value: "default" },
        { key: "persistent", value: "true" },
      ]);
      expect(group.volume).toEqual(VolumePolicy.fromPartial({
        vid: "myapp-pgdata",
        reclaim: VolumePolicy_ReclaimPolicy.retain,
        retention: {},
        maxAttachments: 1,
      }));
    });

    it("compiles a bucket into a group that is never attached and asks for bucket providers", () => {
      const { result } = setup({ sdl: createVolumeSdl({ access: "bucket" }) });
      const [group] = result.groupSpecs;

      expect(group.requirements?.attributes).toEqual([
        { key: "capabilities/storage/buckets", value: "true" },
        { key: "region", value: "us-west" },
      ]);
      expect(group.volume).toEqual(expect.objectContaining({ maxAttachments: 0, access: VolumePolicy_Access.bucket }));
      expect(result.groups).toEqual([Group.fromPartial({ name: "us-west", services: [] })]);
    });

    it("compiles an explicit mount access as a disk", () => {
      const { result } = setup({ sdl: createVolumeSdl({ access: "mount" }) });

      expect(result.groupSpecs[0].volume).toEqual(expect.objectContaining({ maxAttachments: 1, access: VolumePolicy_Access.mount }));
    });

    it("resolves adopt to the owner's volume of the same name", () => {
      const { result } = setup({ sdl: createVolumeSdl({ adopt: { dseq: 555 } }), owner });

      expect(result.groupSpecs[0].volume?.adopt).toEqual({ owner, dseq: 555n, gseq: 1, name: "myapp-pgdata" });
      expect(result.groupSpecs[0].volume?.replicaOf).toBeUndefined();
    });

    it("resolves replica-of to the owner's named volume", () => {
      const { result } = setup({ sdl: createVolumeSdl({ "replica-of": { dseq: 556, name: "other-name" } }), owner });

      expect(result.groupSpecs[0].volume?.replicaOf).toEqual({ owner, dseq: 556n, gseq: 1, name: "other-name" });
      expect(result.groupSpecs[0].volume?.adopt).toBeUndefined();
    });

    it("attaches an external volume through the resource unit and the storage params", () => {
      const { result } = setup({ sdl: createAttachSdl(), owner });
      const [unit] = result.groupSpecs[0].resources;

      expect(result.groupSpecs[0].volume).toBeUndefined();
      expect(unit.volumes).toEqual([{ owner, dseq: 1234567n, gseq: 1, name: "myapp-pgdata" }]);
      expect(unit.resource?.storage).toEqual([]);
      expect(result.groups[0].services[0].params?.storage).toEqual([
        { name: "data", mount: "/var/lib/postgresql/data", readOnly: false, volume: `${owner}/1234567/1/myapp-pgdata` },
      ]);
    });

    it("prefers the owner named by the reference over the owner option", () => {
      const referenceOwner = "akash1365yvmc4s7awdyj3n2sav7xfx76adc6dnmlx63";
      const { result } = setup({ sdl: createAttachSdl({ "myapp-pgdata": { owner: referenceOwner, dseq: 1234567 } }), owner });

      expect(result.groupSpecs[0].resources[0].volumes[0].owner).toBe(referenceOwner);
      expect(result.groups[0].services[0].params?.storage[0].volume).toBe(`${referenceOwner}/1234567/1/myapp-pgdata`);
    });

    it("orders a unit's volume references by their canonical string form", () => {
      const sdl = createAttachSdl(
        { "aa-vol": { dseq: 9 }, "zz-vol": { dseq: 10 } },
        { a: { mount: "/a", volume: "aa-vol" }, z: { mount: "/z", volume: "zz-vol" } },
      );
      const { result } = setup({ sdl, owner });

      expect(result.groupSpecs[0].resources[0].volumes.map(({ name }) => name)).toEqual(["zz-vol", "aa-vol"]);
    });

    it("returns validation errors when an attached volume's owner is unknown", () => {
      const result = generateManifest(createAttachSdl());

      expect(result.ok).toBe(false);
      expect(result.value).toContainEqual(expect.objectContaining({
        message: expect.stringContaining("owner is unknown"),
      }));
    });

    function createVolumeSdl(volume: Record<string, unknown> = {}): SDLInput {
      return yaml`
        version: "2.2"
        volumes:
          myapp-pgdata: ${{ size: "10Gi", class: "beta2", ...volume }}
        profiles:
          placement:
            us-west:
              attributes:
                region: us-west
              pricing:
                myapp-pgdata:
                  denom: uakt
                  amount: 150
        deployment:
          myapp-pgdata:
            us-west:
              profile: myapp-pgdata
              count: 1
      `;
    }

    function createAttachSdl(
      volumes: Record<string, { owner?: string; dseq: number }> = { "myapp-pgdata": { dseq: 1234567 } },
      storage: Record<string, { mount: string; volume: string }> = { data: { mount: "/var/lib/postgresql/data", volume: "myapp-pgdata" } },
    ): SDLInput {
      const externalVolumes = Object.fromEntries(Object.entries(volumes).map(([name, external]) => [name, { external }]));

      return yaml`
        version: "2.2"
        volumes: ${externalVolumes}
        services:
          db:
            image: postgres:16
            params:
              storage: ${storage}
        profiles:
          compute:
            db:
              resources:
                cpu:
                  units: 2
                memory:
                  size: 4Gi
                storage: []
          placement:
            anywhere:
              pricing:
                db:
                  denom: uakt
                  amount: 800
        deployment:
          db:
            anywhere:
              profile: db
              count: 1
      `;
    }
  });

  function setup(input?: {
    sdl: SDLInput;
    owner?: string;
  }) {
    const result = generateManifest(input?.sdl ?? createBasicSdl(), { owner: input?.owner });
    assertBuildResult(result);
    return { result: result.value };
  }

  function assertBuildResult(result: ReturnType<typeof generateManifest>): asserts result is Extract<GenerateManifestResult, { ok: true }> {
    if (!result.ok) {
      throw new Error(`Expected BuildResult but got validation errors: ${JSON.stringify(result)}`);
    }
  }

  function createBasicSdl(input: {
    image?: string;
    port?: number;
    cpu?: number | string;
    memory?: string;
    storage?: SDLCompute["resources"]["storage"];
    gpu?: SDLCompute["resources"]["gpu"];
    denom?: string;
    amount?: number;
    placementAttributes?: SDLInput["profiles"]["placement"][string]["attributes"];
    signedBy?: SDLInput["profiles"]["placement"][string]["signedBy"];
    env?: SDLService["env"];
    command?: SDLService["command"];
    args?: SDLService["args"];
    credentials?: SDLService["credentials"];
    expose?: SDLService["expose"];
    endpoints?: SDLInput["endpoints"];
    reclamation?: SDLInput["reclamation"];
  } = {}): SDLInput {
    const {
      port = 80,
      cpu = 0.5,
      memory = "512Mi",
      storage = "512Mi",
      gpu,
      denom = "uakt",
      amount = 1000,
    } = input;

    return yaml`
      version: "2.1"
      services:
        web:
          image: ${input.image ?? "nginx"}
          env: ${input.env}
          command: ${input.command}
          args: ${input.args}
          credentials: ${input.credentials}
          expose: ${input.expose ?? [{ port, as: port, to: [{ global: true }] }]}
      profiles:
        compute:
          web:
            resources:
              cpu:
                units: ${cpu}
              memory:
                size: ${memory}
              storage: ${typeof storage === "string" ? { size: storage } : storage}
              gpu: ${gpu}
        placement:
          dcloud:
            pricing:
              web: ${{ denom, amount }}
            attributes: ${input.placementAttributes}
            signedBy: ${input.signedBy}
      deployment:
        web:
          dcloud:
            profile: web
            count: 1
      endpoints: ${input.endpoints}
      reclamation: ${input.reclamation}
    `;
  }

  type RelativeFilePath<T extends string> = string & { baseDir?: T };
  function readFileSync(filePath: RelativeFilePath<"$dir">): string {
    const fullPath = path.join(__dirname, filePath);
    return fs.readFileSync(fullPath, "utf8");
  }

  function stringifyGroups(manifest: Group[]): string {
    return JSON.stringify(manifest.map((g) => Group.toJSON(g)), null, 2);
  }

  function stringifyGroupSpecs(manifest: GroupSpec[]): string {
    return JSON.stringify(manifest.map((g) => GroupSpec.toJSON(g)), null, 2);
  }
});
