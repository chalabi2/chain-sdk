import { ExportChunk, ExportRequest, StatusRequest, StatusResponse } from "./service.ts";

export const VolumeTransfer = {
  typeName: "akash.volume.v1.VolumeTransfer",
  methods: {
    export: {
      name: "Export",
      kind: "server_streaming",
      input: ExportRequest,
      output: ExportChunk,
      get parent() { return VolumeTransfer; },
    },
    status: {
      name: "Status",
      httpPath: "/v1/volume/status",
      input: StatusRequest,
      output: StatusResponse,
      get parent() { return VolumeTransfer; },
    },
  },
} as const;
