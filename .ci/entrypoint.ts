import { Workspace, pushStep, spawnChildJob, print } from "runtime/core.ts";
import * as Docker from "pkg/buildy/docker@1/mod.ts";
import { uploadArtifact } from "runtime/artifacts.ts";

const PLATFORMS = [
  {os: "linux", arch: "amd64"},
  {os: "windows", arch: "amd64"},
]

export async function build(ws: Workspace, { os, arch, version }: { os?: string; arch?: string; version?: string }) {
  pushStep("Build Jambon Binary");
  const res = await Docker.run("go build -o jambon cmd/jambon/main.go && ls -lah", {
    image: `golang:1.16`,
    copy: ["cmd/**", "tacview/**", "go.sum", "go.mod", "*.go"],
    env: [`GOOS=${os || "linux"}`, `GOARCH=${arch || "amd64"}`]
  });

  if (version !== undefined) {
    await res.copy("/jambon");

    pushStep("Upload Jambon Binary");
    const uploadRes = await uploadArtifact("jambon", {
      name: `jambon-${os}-${arch}-${version}`,
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

const semVerRe = /v([0-9]+)\.([0-9]+)\.([0-9]+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+)?/;

export async function githubPush(ws: Workspace) {
  let version;

  const versionTags = ws.commit.tags.filter((tag) => semVerRe.test(tag));
  if (versionTags.length == 1) {
    print(`Found version tag ${versionTags[0]}, will build release artifacts.`);
    version = versionTags[0]
  } else if (versionTags.length > 1) {
    throw new Error(`Found too many version tags: ${versionTags}`);
  }

  await spawnChildJob(".ci/entrypoint.ts:build", {
    alias: "Build Linux amd64",
    args: { os: "linux", arch: "amd64", version: version }
  })

  await spawnChildJob(".ci/entrypoint.ts:build", {
    alias: "Build Windows amd64",
    args: { os: "windows", arch: "amd64", version: version }
  })
}