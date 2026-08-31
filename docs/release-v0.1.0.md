# v0.1.0 release handoff

The local repository has an explicit three-file direct-main bootstrap recorded
in [`bootstrap-direct-main.json`](bootstrap-direct-main.json). Feature work is
on the `implementation` branch. The required promotion is one human-created
same-repository pull request into `main`, with the GitHub Actions conformance
workflow as the authority, followed by the annotated `v0.1.0` tag.

The release workflow prepares these caller-owned assets and their exact SHA-256
digests:

- `gooo-causal-verification-runner-v0.1.0.tar.gz`
- `SHA256SUMS`
- `release-manifest.json`

The workflow does not create a pull request, mutate a branch, or change
repository settings. It creates the new release only after the annotated tag
is pushed, never rewrites an existing release, and audits the external GitHub
Releases API `immutable=true` plus every asset digest. The repository's
immutable-releases setting must be enabled by the maintainer before tagging.
The manifest's `self_asserted_immutable=true` is not sufficient evidence.
