import { Workspace, pushStep, spawnChildJob, print} from "runtime/core.ts";
import * as Docker from "pkg/buildy/docker@1/mod.ts";
import { uploadArtifact } from "runtime/artifacts.ts";

export async function build(ws: Workspace, { os, arch, version }: { os?: string; arch?: string; version?: string }) {
  pushStep("Build Jambon Binary");
  const res = await Docker.run("go build -o jambon cmd/jambon/main.go && ls -lah", {
    image: `golang:1.16`,
    copy: ["cmd/**", "tacview/**", "go.sum", "go.mod", "*.go"],
    env: [`GOOS=${os || "linux"}`, `GOARCH=${arch || "amd64"}`]
  });

  if (version !== undefined) {
    await res.copy("/tacview");
   
    pushStep("Upload Jambon Binary");
    const uploadRes = await uploadArtifact("tacview", {
      name: "tacview",
      published: true,
      labels: [
        "jambon",
        `arch:${arch}`,
        `os:${os}`,
        `version:${version}`,
      ],
    });
    print(
      `Uploaded binary to ${uploadRes.generatePublicURL(
        ws.org,
        ws.repository
      )}`
    );

  }

}

export async function githubPush(ws: Workspace) {
  await spawnChildJob(".ci/entrypoint.ts:build", {
    alias: "Build Linux amd64",
    args: {os: "linux", arch: "amd64"}
  })
  
  await spawnChildJob(".ci/entrypoint.ts:build", {
    alias: "Build Windows amd64",
    args: {os: "windows", arch: "amd64"}
  })
}