import { VolumePolicy, VolumePolicy_Access, VolumePolicy_ReclaimPolicy, VolumeRef } from "../../generated/protos/akash/deployment/v1/volume.ts";
import { Duration } from "../../generated/protos/google/protobuf/duration.ts";
import { parseDuration } from "../duration.ts";
import type { SDLInput } from "../validateSDL/validateSDL.ts";
import type { VolumeDeclaration, VolumeExternalRef, VolumePolicyRef } from "../validateSDL/validateSDLInput.ts";
import { compareStrings, type SDLService } from "./manifestUtils.ts";

export type SDLVolume = VolumeExternalRef | VolumeDeclaration;

/**
 * Placement capability injected into a volume group's requirements so only
 * providers that opted into the decoupled storage market match the order.
 */
export const STORAGE_CAPABILITY_VOLUMES = "capabilities/storage/volumes";

/**
 * Placement capability a bucket group injects instead, so only providers
 * that serve S3 buckets match the order.
 */
export const STORAGE_CAPABILITY_BUCKETS = "capabilities/storage/buckets";

/** A bucket is reached over the network by key holders rather than attached. */
export function isBucketVolume(volume: VolumeDeclaration): boolean {
  return volume.access === "bucket";
}

/** Mirrors the capability choice in `buildVolumeGroup` in go/sdl/groupBuilder_v2_2.go. */
export function volumeCapability(volume: VolumeDeclaration): string {
  return isBucketVolume(volume) ? STORAGE_CAPABILITY_BUCKETS : STORAGE_CAPABILITY_VOLUMES;
}

export function isExternalVolume(volume: SDLVolume): volume is VolumeExternalRef {
  return "external" in volume;
}

export function declaredVolumeNames(sdl: SDLInput): string[] {
  return Object.entries(sdl.volumes ?? {})
    .filter(([, volume]) => !isExternalVolume(volume))
    .map(([name]) => name)
    .sort(compareStrings);
}

export function serviceVolumeNames(service: SDLService): string[] {
  return Object.values(service.params?.storage ?? {})
    .flatMap((params) => (params.volume ? [params.volume] : []));
}

/**
 * Mirrors `VolumeRef.String()`; the manifest carries a volume reference in
 * this form and the provider matches it against the group spec's refs.
 */
export function volumeRefToString(ref: VolumeRef): string {
  return `${ref.owner}/${ref.dseq}/${ref.gseq}/${ref.name}`;
}

/**
 * Resolves every external volume whose owner is known. gseq is always 1: a
 * volume deployment holds exactly one group.
 */
export function externalVolumeRefs(sdl: SDLInput, owner: string | undefined): Map<string, VolumeRef> {
  const refs = new Map<string, VolumeRef>();

  for (const [name, volume] of Object.entries(sdl.volumes ?? {})) {
    if (!isExternalVolume(volume)) continue;

    const refOwner = volume.external.owner || owner;
    if (!refOwner) continue;

    refs.set(name, VolumeRef.fromPartial({ owner: refOwner, dseq: BigInt(volume.external.dseq), gseq: 1, name }));
  }

  return refs;
}

export function serviceVolumeRefs(service: SDLService, refs: Map<string, VolumeRef>): VolumeRef[] {
  return serviceVolumeNames(service)
    .map((name) => refs.get(name)!)
    .sort((a, b) => compareStrings(volumeRefToString(a), volumeRefToString(b)));
}

/**
 * The lifecycle contract travels in the typed policy, never in storage
 * attributes: attributes take part in provider matching, and a tenant-unique
 * value there would make the order unbiddable.
 */
export function buildVolumePolicy(name: string, volume: VolumeDeclaration, owner: string | undefined): VolumePolicy {
  return VolumePolicy.fromPartial({
    vid: name,
    reclaim: volume.lifecycle?.reclaim === "delete" ? VolumePolicy_ReclaimPolicy.delete : VolumePolicy_ReclaimPolicy.retain,
    retention: retentionToDuration(volume.lifecycle?.retention),
    maxAttachments: isBucketVolume(volume) ? 0 : 1,
    maxReplicas: volume["max-replicas"] ?? 0,
    access: isBucketVolume(volume) ? VolumePolicy_Access.bucket : VolumePolicy_Access.mount,
    adopt: volume.adopt && policyRef(name, volume.adopt, owner),
    replicaOf: volume["replica-of"] && policyRef(name, volume["replica-of"], owner),
  });
}

function policyRef(volumeName: string, ref: VolumePolicyRef, owner: string | undefined): VolumeRef {
  return VolumeRef.fromPartial({ owner, dseq: BigInt(ref.dseq), gseq: 1, name: ref.name ?? volumeName });
}

function retentionToDuration(retention: string | undefined): Duration {
  const milliseconds = retention === undefined ? 0n : parseDuration(retention)?.milliseconds ?? 0n;
  return Duration.fromPartial({ seconds: milliseconds / 1_000n, nanos: 0 });
}
