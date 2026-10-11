---
name: release
description: Release a new version of go-mail-webhook-service. Supports two release types — full (new image + chart) and chart-only (chart bump, existing image). Uses a PR-based flow so CI runs green on a PR before anything is tagged or published. Run in foreground (run_in_background: false) so step progress is visible in real time.
allowedTools:
  - Read
  - Edit
  - Bash(git *)
  - Bash(gh *)
  - Bash(helm *)
  - Bash(grep *)
  - Bash(go *)
  - Bash(timeout *)
  - Bash(sleep *)
  - Bash(for *)
  - Bash(seq *)
---

## Release process for go-mail-webhook-service

Follow these steps in order. Do not skip steps. After each step, report completion with a one-line status so the user can track progress.

> **Core principle — the merge gate comes FIRST.** Code and chart changes land on
> `main` only through a **pull request whose CI is green and which has been
> merged**. Nothing is tagged or published from a dirty or unmerged working tree.
> The chart version bump is committed onto the release branch *within that same
> PR* so CI validates the bumped chart before the merge completes. Determine the
> version first (Step 1), open the gated PR (Step 2) with the bump committed to
> the branch (Step 3) before the merge, then tag/publish (Steps 3b–5).

### Polling rules (apply to EVERY `gh` wait in this process — never let a step hang)

Obey these everywhere you wait on CI:

- **Never use the jq `//` default operator in `-q`** (e.g. `.conclusion // "pending"`).
  Under Git-Bash the `//` is rewritten by MSYS path conversion and the whole
  expression breaks, so every poll returns a parse error and the loop sleeps its
  full ceiling. Query `status` and `conclusion` as **separate** `--json` fields
  and default in the shell with `${var:-pending}`.
- **Wrap every `gh` call in `timeout 25 gh ...`** so one hung network call can't
  stall the loop.
- **Hard iteration cap** — at most 12 iterations, 30s apart (~6 min ceiling).
  Always `break` at the cap; never loop unbounded.
- **Run long waits as a background task** (`run_in_background: true`) so the turn
  is never blocked for minutes. Read the task's output file to collect the
  result, or react to its completion notification.
- Treat an empty/`null` conclusion as `pending`, never as an error.

Reusable status snippet (status + conclusion as separate fields, no `//`):
```bash
S=$(timeout 25 gh run view "$RUN_ID" --repo jo-hoe/go-mail-webhook-service --json status -q .status 2>/dev/null)
C=$(timeout 25 gh run view "$RUN_ID" --repo jo-hoe/go-mail-webhook-service --json conclusion -q .conclusion 2>/dev/null)
echo "image: ${S:-?}/${C:-pending}"
```

### Step 1 — Determine version and release type

Report: `[Step 1/5] Determining version and release type...`

Check the current versions and what has changed since the last tag:
```bash
grep -E '^version:|^appVersion:' charts/go-mail-webhook-service/Chart.yaml
git tag --sort=-v:refname | head -3
git log $(git tag --sort=-v:refname | head -1)..HEAD --oneline
git diff $(git tag --sort=-v:refname | head -1)..HEAD -- charts/go-mail-webhook-service/ *.go app/ cli/ go.mod go.sum Dockerfile
git status --short   # also inspect the uncommitted working tree
```

**Determine release type from the diff AND the working tree:**
- **No release needed** — only non-app, non-chart files changed (e.g. `.claude/`, `.github/`, `docs/`, `Makefile`, `README.md`). Skip the bump and tag, but still land any uncommitted working-tree changes via the Step 2 PR flow (no version bump in that PR). Report: `[Step 1/5] No version bump needed — opening PR for non-release changes.`
- **Full release** — app code changed (root `*.go`, `app/`, `cli/`, `go.mod`, `go.sum`, `Dockerfile`). Bumps both `version` and `appVersion`. Pushes a new semver tag to trigger the Docker image build (`Release Image` workflow).
- **Chart-only release** — only chart templates/values changed (`charts/`), no app code changes. Bumps only `version`, keeps `appVersion`. No new tag pushed.

Confirm the target version with the user if it is not obvious from the changes.

Report: `[Step 1/5] ✓ Release type: <full|chart-only>, chart version: <new-version>, appVersion: <app-version>`

### Step 2 — Open a PR, wait for green CI, and merge (the gate)

Report: `[Step 2/5] Opening release PR...`

> This is the gate: code lands on `main` only through a green, merged PR. Tagging
> and publishing (Steps 3b–4) happen only AFTER the merge. The Step 3 Chart.yaml
> bump is committed onto this branch *before* the merge, so CI validates it.

**2.0 — Verify locally before pushing.** Run the project's build + tests so the PR
starts from a known-good state:
```bash
go build ./... && go test ./...
```
Fix any failure before continuing — do not open a PR on red local tests.

**2.1 — Branch.** If the working tree already sits on `main`, create a release
branch so the changes go through a PR rather than a direct push:
```bash
git fetch origin
git switch -c release/v<new-version> origin/main   # or reuse an existing feature branch
```

**2.2 — Commit the app/chart changes.** Stage and commit the release changes
(app/test/chart edits) on the branch. Use `git diff --cached` and
`git log --oneline -5` to write an accurate message:
```bash
git add <changed files>
git commit -m "feat: <summary of the change>"
```

**2.3 — Apply the Step 3 bump on this branch, then push.** Perform the Chart.yaml
(and chart README badge) bump described in Step 3 now, commit it, and push the
branch. The bump rides along in this PR so CI validates the bumped chart:
```bash
git add charts/go-mail-webhook-service/Chart.yaml charts/go-mail-webhook-service/README.md
git commit -m "chore: bump chart and appVersion to <new-version>"
git push -u origin release/v<new-version>
gh pr create --repo jo-hoe/go-mail-webhook-service --base main --fill \
  --title "release: v<new-version>" \
  --body "Release v<new-version>. <one-line summary of what changed.>"
PR=$(gh pr view --repo jo-hoe/go-mail-webhook-service --json number -q .number)
echo "PR=#$PR"
```

**2.4 — Wait for PR CI (`lint`, `test`) to go green, then merge.** Run the wait as a
**background task** so the turn is not blocked (≤12 polls, 30s apart,
status/conclusion as separate fields, no jq `//`):
```bash
for i in $(seq 1 12); do
  STATE=$(timeout 25 gh pr view "$PR" --repo jo-hoe/go-mail-webhook-service --json state -q .state 2>/dev/null)
  CHECKS=$(timeout 25 gh pr view "$PR" --repo jo-hoe/go-mail-webhook-service --json statusCheckRollup -q '[.statusCheckRollup[].conclusion] | join(",")' 2>/dev/null)
  echo "[Step 2/5] Poll $i/12 — PR state: ${STATE:-?}, checks: ${CHECKS:-pending}"
  if echo "$CHECKS" | grep -qiE 'failure|cancelled|timed_out'; then
    echo "[Step 2/5] ✗ PR checks failed: $CHECKS"
    gh pr checks "$PR" --repo jo-hoe/go-mail-webhook-service 2>&1 | tail -20
    exit 1
  fi
  # merge once all checks are green and none pending
  if [ -n "$CHECKS" ] && ! echo "$CHECKS" | grep -qiE 'pending|,$' ; then
    gh pr merge "$PR" --repo jo-hoe/go-mail-webhook-service --merge && { echo "MERGED"; break; }
  fi
  [ "$i" -eq 12 ] && { echo "[Step 2/5] ✗ ceiling reached without merge"; exit 1; }
  sleep 30
done
```
If checks fail, report the failing check and stop — do NOT tag or publish.

After the merge, sync local `main`:
```bash
git switch main && git pull --ff-only origin main
```

Report: `[Step 2/5] ✓ PR #<n> merged to main (CI green)`

> **Chart-only release:** the merge to `main` is the only trigger needed — the
> `Release Charts` workflow fires on the `charts/go-mail-webhook-service/Chart.yaml`
> change. Skip Step 3b (no new image) and go to Step 4.

### Step 3 — Chart.yaml bump (performed inside the Step 2 PR)

Report: `[Step 3/5] Bumping Chart.yaml (committed on the release branch)...`

This bump is NOT a separate commit to `main`. It is the edit staged in Step 2.3
so CI validates it inside the PR. Edit `charts/go-mail-webhook-service/Chart.yaml`:

**Full release:** update both fields:
```yaml
version: <new-version>
appVersion: "<new-version>"
```

**Chart-only release:** update only `version`, leave `appVersion` unchanged:
```yaml
version: <new-version>
appVersion: "<current-app-version>"   # unchanged
```

Also bump any version/appVersion badges at the top of
`charts/go-mail-webhook-service/README.md` to match (they are otherwise easy to
leave stale).

Report: `[Step 3/5] ✓ Chart.yaml updated (validated by the Step 2 PR CI)`

### Step 3b — Full release only: tag merged main, wait for the image, confirm

Report: `[Step 3b/5] Tagging merged main and waiting for image build...`

Skip this step entirely for chart-only releases.

> **Ordering invariant:** the chart's `appVersion` makes the deployment pull
> `ghcr.io/jo-hoe/go-mail-webhook-service:<appVersion>`. That image is built
> **only** by the `v<new-version>` tag push (`Release Image` workflow). Tag
> immediately after merge and confirm the image before telling the user the
> release is done.

Tag the merged `main` HEAD and push the tag:
```bash
git switch main && git pull --ff-only origin main
git tag v<new-version>
git push origin v<new-version>        # triggers the Release Image workflow
```

Wait for the image build (the `Release Image` run on the `v<new-version>` tag ref)
to reach `completed/success`, as a **background task**, obeying the polling rules:
```bash
RUN_ID=$(timeout 25 gh run list --repo jo-hoe/go-mail-webhook-service --branch v<new-version> \
  --workflow "Release Image" --limit 1 --json databaseId -q '.[0].databaseId')
for i in $(seq 1 12); do
  S=$(timeout 25 gh run view "$RUN_ID" --repo jo-hoe/go-mail-webhook-service --json status -q .status 2>/dev/null)
  C=$(timeout 25 gh run view "$RUN_ID" --repo jo-hoe/go-mail-webhook-service --json conclusion -q .conclusion 2>/dev/null)
  echo "[Step 3b/5] Poll $i/12 — image: ${S:-?}/${C:-pending}"
  [ "$S" = "completed" ] && [ "$C" = "success" ] && { echo "IMAGE_OK"; break; }
  if [ "$S" = "completed" ] && [ "$C" != "success" ]; then
    echo "image build failed: $C"; gh run view "$RUN_ID" --repo jo-hoe/go-mail-webhook-service --log-failed 2>&1 | tail -40; exit 1
  fi
  [ "$i" -eq 12 ] && { echo "[Step 3b/5] ✗ ceiling reached"; exit 1; }
  sleep 30
done
```

Report: `[Step 3b/5] ✓ Image v<new-version> built`

### Step 4 — Babysit chart publish

Report: `[Step 4/5] Waiting for Release Charts (bounded)...`

The merge to `main` triggered `Release Charts`. Track it with a bounded,
backgrounded poll (polling rules apply; status/conclusion as separate fields):
```bash
for i in $(seq 1 12); do
  rc_s=$(timeout 25 gh run list --repo jo-hoe/go-mail-webhook-service --workflow "Release Charts" --limit 1 --json status -q '.[0].status' 2>/dev/null)
  rc_c=$(timeout 25 gh run list --repo jo-hoe/go-mail-webhook-service --workflow "Release Charts" --limit 1 --json conclusion -q '.[0].conclusion' 2>/dev/null)
  echo "[Step 4/5] Poll $i/12 — chart: ${rc_s:-?}/${rc_c:-pending}"
  [ "$rc_s" = "completed" ] && { echo "CHART=$rc_c"; break; }
  [ "$i" -eq 12 ] && { echo "[Step 4/5] ✗ ceiling reached"; break; }
  sleep 30
done
```
If `Release Charts` shows `failure`, fetch logs and stop:
```bash
timeout 60 gh run view <id> --repo jo-hoe/go-mail-webhook-service --log-failed
```

Note: `chart-releaser` publishes to `gh-pages`, which then triggers
`pages build and deployment`. The chart is not live on the Pages site until that
Pages deployment completes — allow for it in Step 5.

Report: `[Step 4/5] ✓ Chart publish completed`

### Step 5 — Verify and confirm

Report: `[Step 5/5] Verifying published artifacts...`

Verify the chart was published as a GitHub release by `chart-releaser` (created
before the Pages deployment, so it's the most reliable signal):
```bash
gh release view go-mail-webhook-service-<new-version> --repo jo-hoe/go-mail-webhook-service
```

Then confirm it is served from the Pages Helm repo (may lag until the Pages
deployment finishes — retry a few times if needed):
```bash
helm repo add go-mail-webhook-service https://jo-hoe.github.io/go-mail-webhook-service 2>/dev/null; helm repo update go-mail-webhook-service
helm search repo go-mail-webhook-service/go-mail-webhook-service --version <new-version>
```

For full releases, the image tag is confirmed by the image build succeeding in
Step 3b.

If chart verification fails, report the error and stop.

Report: `[Step 5/5] ✓ Release complete`

Confirm:
- PR: `#<n>` merged to main with green CI
- Chart release: `go-mail-webhook-service-<new-version>` on jo-hoe/go-mail-webhook-service
- Helm repo: `https://jo-hoe.github.io/go-mail-webhook-service` → `go-mail-webhook-service/go-mail-webhook-service --version <new-version>`
- Image: `ghcr.io/jo-hoe/go-mail-webhook-service:<new-version>` (full release) or `unchanged at <current-app-version>` (chart-only)
