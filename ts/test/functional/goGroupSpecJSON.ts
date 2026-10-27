import type { PlacementRequirements } from "../../src/generated/protos/akash/base/attributes/v1/attribute.ts";
import type { Endpoint } from "../../src/generated/protos/akash/base/resources/v1beta4/endpoint.ts";
import type { Resources } from "../../src/generated/protos/akash/base/resources/v1beta4/resources.ts";
import type { VolumePolicy, VolumeRef } from "../../src/generated/protos/akash/deployment/v1/volume.ts";
import type { GroupSpec } from "../../src/generated/protos/akash/deployment/v1beta4/groupspec.ts";
import type { ResourceUnit } from "../../src/generated/protos/akash/deployment/v1beta4/resourceunit.ts";
import type { Duration } from "../../src/generated/protos/google/protobuf/duration.ts";

/**
 * Renders TypeScript group specs the way Go's `json.Marshal` renders
 * `dtypes.GroupSpecs`, so they can be compared against the committed
 * `group-specs.json` fixtures the Go generator produces.
 *
 * The two shapes differ in ways that no generic key mapping can express, so the
 * mapping is spelled out field by field. The rules come from the gogoproto json
 * tags in `go/node/**`:
 *
 *   - `signed_by` / `all_of` / `any_of` / `size` / `sequence_number` /
 *     `max_attachments` / `max_replicas` / `replica_of` are custom json tags,
 *     not the camelCase ts-proto derives from the field names.
 *   - `PlacementRequirements.Attributes` has NO `omitempty`, so an empty list
 *     marshals as `null`. Every resource-level `Attributes` (cpu, gpu, memory,
 *     storage) DOES have `omitempty`, so an empty list is dropped entirely.
 *   - `Resources.Storage` and `ResourceUnit.Volumes` are `omitempty`, so a unit
 *     that only attaches external volumes has neither key, and a unit that
 *     attaches none has no `volumes`.
 *   - `GroupSpec.Volume` and `VolumePolicy.Adopt`/`ReplicaOf` are `omitempty`
 *     pointers, present only on storage-only (volume) groups.
 *   - `VolumePolicy.Retention` is a `time.Duration`, which marshals as int64
 *     nanoseconds; `VolumeRef.DSeq` is a uint64, a plain JSON number.
 *   - `SignedBy.AllOf`/`AnyOf` are nil slices when unset, which marshal as `null`.
 *   - `Endpoint.Kind` has `omitempty`, so SHARED_HTTP (0) is dropped.
 *   - `ResourceValue.Val` is a `cosmossdk.io/math.Int`, which marshals as a
 *     quoted decimal string; on this side it is a bigint.
 *
 * Prices are left as-is — `sdl-parity.spec.ts` runs both sides through
 * `LegacyDec.encode`, since Go writes an 18-decimal `DecCoin` amount
 * ("250.000000000000000000") where this side carries the plain "250".
 */
export function toGoGroupSpecJSON(specs: GroupSpec[]): unknown {
  return specs.map((spec) => ({
    name: spec.name,
    requirements: requirementsToGoJSON(spec.requirements),
    resources: spec.resources.map(resourceUnitToGoJSON),
    ...(spec.volume ? { volume: volumePolicyToGoJSON(spec.volume) } : {}),
  }));
}

function requirementsToGoJSON(requirements: PlacementRequirements | undefined) {
  return {
    signed_by: {
      all_of: nullWhenEmpty(requirements?.signedBy?.allOf),
      any_of: nullWhenEmpty(requirements?.signedBy?.anyOf),
    },
    attributes: nullWhenEmpty(requirements?.attributes),
  };
}

function resourceUnitToGoJSON(unit: ResourceUnit) {
  return {
    resource: resourcesToGoJSON(unit.resource!),
    count: unit.count,
    price: unit.price,
    ...omitWhenEmpty("volumes", unit.volumes.map(volumeRefToGoJSON)),
  };
}

function resourcesToGoJSON(resources: Resources) {
  return {
    id: resources.id,
    cpu: {
      units: { val: String(resources.cpu!.units!.val) },
      ...omitWhenEmpty("attributes", resources.cpu!.attributes),
    },
    memory: {
      size: { val: String(resources.memory!.quantity!.val) },
      ...omitWhenEmpty("attributes", resources.memory!.attributes),
    },
    ...omitWhenEmpty("storage", resources.storage.map((volume) => ({
      name: volume.name,
      size: { val: String(volume.quantity!.val) },
      ...omitWhenEmpty("attributes", volume.attributes),
    }))),
    gpu: {
      units: { val: String(resources.gpu!.units!.val) },
      ...omitWhenEmpty("attributes", resources.gpu!.attributes),
    },
    endpoints: resources.endpoints.map(endpointToGoJSON),
  };
}

function endpointToGoJSON(endpoint: Endpoint) {
  return {
    ...(endpoint.kind === 0 ? {} : { kind: endpoint.kind }),
    sequence_number: endpoint.sequenceNumber,
  };
}

function volumePolicyToGoJSON(policy: VolumePolicy) {
  return {
    vid: policy.vid,
    reclaim: policy.reclaim,
    retention: durationToGoJSON(policy.retention),
    max_attachments: policy.maxAttachments,
    max_replicas: policy.maxReplicas,
    ...(policy.adopt ? { adopt: volumeRefToGoJSON(policy.adopt) } : {}),
    ...(policy.replicaOf ? { replica_of: volumeRefToGoJSON(policy.replicaOf) } : {}),
    ...(policy.access ? { access: policy.access } : {}),
  };
}

function volumeRefToGoJSON(ref: VolumeRef) {
  return {
    owner: ref.owner,
    dseq: Number(ref.dseq),
    gseq: ref.gseq,
    name: ref.name,
  };
}

function durationToGoJSON(duration: Duration | undefined): number {
  return Number((duration?.seconds ?? 0n) * 1_000_000_000n + BigInt(duration?.nanos ?? 0));
}

function nullWhenEmpty<T>(values: T[] | undefined): T[] | null {
  return values?.length ? values : null;
}

function omitWhenEmpty<T>(key: string, values: T[] | undefined) {
  return values?.length ? { [key]: values } : {};
}
