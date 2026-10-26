package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kopia/kopia/cli"
	"github.com/kopia/kopia/internal/testutil"
	"github.com/kopia/kopia/tests/testenv"
)

func TestSnapshotMigrate(t *testing.T) {
	t.Parallel()

	runner := testenv.NewInProcRunner(t)

	sourceEnv := testenv.NewCLITest(t, testenv.RepoFormatNotImportant, runner)
	defer sourceEnv.RunAndExpectSuccess(t, "repo", "disconnect")

	sourceEnv.RunAndExpectSuccess(t, "repo", "create", "filesystem", "--path", sourceEnv.RepoDir)

	srcdir := testutil.TempDirectory(t)
	require.NoError(t, os.WriteFile(filepath.Join(srcdir, "some-file2"), []byte{1, 2, 3}, 0o755))

	// Take a snapshot on the source repository
	sourceEnv.RunAndExpectSuccess(t, "snapshot", "create", "--tags", "one:1", "--tags", "two:2", srcdir)

	// Verify the tags were stored in the manifest
	var snapshotsSource []*cli.SnapshotManifest
	testutil.MustParseJSONLines(t, sourceEnv.RunAndExpectSuccess(t, "snapshot", "list",
		"--json"), &snapshotsSource)
	require.Len(t, snapshotsSource, 1)

	require.Equal(t, "1", snapshotsSource[0].Tags["tag:one"])
	require.Equal(t, "2", snapshotsSource[0].Tags["tag:two"])

	destEnv := testenv.NewCLITest(t, testenv.RepoFormatNotImportant, runner)
	defer destEnv.RunAndExpectSuccess(t, "repo", "disconnect")

	destEnv.RunAndExpectSuccess(t, "repo", "create", "filesystem", "--path", destEnv.RepoDir)

	// Migrate the snapshot to the destination repository
	destEnv.RunAndExpectSuccess(
		t,
		"snapshot", "migrate",
		"--tags", "tag:one",
		"--tags", "tag:two",
		"--all",
		"--source-config", filepath.Join(sourceEnv.ConfigDir, ".kopia.config"),
	)

	// Verify the tags have been migrated to the destination repository
	var snapshotsDestination []*cli.SnapshotManifest
	testutil.MustParseJSONLines(t, destEnv.RunAndExpectSuccess(t, "snapshot", "list",
		"--json"), &snapshotsDestination)

	require.Len(t, snapshotsDestination, 1)
	require.Equal(t, "1", snapshotsDestination[0].Tags["tag:one"])
	require.Equal(t, "2", snapshotsDestination[0].Tags["tag:two"])
}

func TestSnapshotMigrateMatchTags(t *testing.T) {
	t.Parallel()

	runner := testenv.NewInProcRunner(t)

	sourceEnv := testenv.NewCLITest(t, testenv.RepoFormatNotImportant, runner)
	defer sourceEnv.RunAndExpectSuccess(t, "repo", "disconnect")

	sourceEnv.RunAndExpectSuccess(t, "repo", "create", "filesystem", "--path", sourceEnv.RepoDir)

	srcdir := testutil.TempDirectory(t)

	for i, tags := range [][]string{
		{"env:prod", "tier:db"},
		{"env:prod", "tier:web"},
		{"env:dev", "tier:db"},
	} {
		require.NoError(t, os.WriteFile(filepath.Join(srcdir, "file"+string(rune('a'+i))), []byte{byte(i)}, 0o755))

		args := []string{"snapshot", "create", srcdir}
		for _, tag := range tags {
			args = append(args, "--tags", tag)
		}

		sourceEnv.RunAndExpectSuccess(t, args...)
	}

	var sourceSnapshots []*cli.SnapshotManifest

	testutil.MustParseJSONLines(t, sourceEnv.RunAndExpectSuccess(t, "snapshot", "list", "--json"), &sourceSnapshots)
	require.Len(t, sourceSnapshots, 3)

	destEnv := testenv.NewCLITest(t, testenv.RepoFormatNotImportant, runner)
	defer destEnv.RunAndExpectSuccess(t, "repo", "disconnect")

	destEnv.RunAndExpectSuccess(t, "repo", "create", "filesystem", "--path", destEnv.RepoDir)

	// only the first snapshot has both tags
	destEnv.RunAndExpectSuccess(
		t,
		"snapshot", "migrate",
		"--all",
		"--match-tags", "env:prod",
		"--match-tags", "tier:db",
		"--source-config", filepath.Join(sourceEnv.ConfigDir, ".kopia.config"),
	)

	var destSnapshots []*cli.SnapshotManifest

	testutil.MustParseJSONLines(t, destEnv.RunAndExpectSuccess(t, "snapshot", "list", "--json"), &destSnapshots)
	require.Len(t, destSnapshots, 1)
	// object IDs differ across repositories, compare the directory contents instead
	require.EqualValues(t, 1, sourceSnapshots[0].RootEntry.DirSummary.TotalFileCount)
	require.Equal(t, sourceSnapshots[0].RootEntry.DirSummary.TotalFileCount, destSnapshots[0].RootEntry.DirSummary.TotalFileCount)
}

func TestSnapshotMigrateMatchTagsInvalid(t *testing.T) {
	t.Parallel()

	runner := testenv.NewInProcRunner(t)

	sourceEnv := testenv.NewCLITest(t, testenv.RepoFormatNotImportant, runner)
	defer sourceEnv.RunAndExpectSuccess(t, "repo", "disconnect")

	sourceEnv.RunAndExpectSuccess(t, "repo", "create", "filesystem", "--path", sourceEnv.RepoDir)

	destEnv := testenv.NewCLITest(t, testenv.RepoFormatNotImportant, runner)
	defer destEnv.RunAndExpectSuccess(t, "repo", "disconnect")

	destEnv.RunAndExpectSuccess(t, "repo", "create", "filesystem", "--path", destEnv.RepoDir)

	destEnv.RunAndExpectFailure(
		t,
		"snapshot", "migrate",
		"--all",
		"--match-tags", "env",
		"--source-config", filepath.Join(sourceEnv.ConfigDir, ".kopia.config"),
	)
}
