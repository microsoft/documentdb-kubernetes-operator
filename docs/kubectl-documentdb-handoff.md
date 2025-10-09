# Kubectl DocumentDB Plugin Hand-off

## Build the Plugin Binary

The repository already vendors the kubectl DocumentDB plugin. Rebuild it from source to ensure you hand off a fresh binary:

```bash
cd plugins/documentdb-kubectl-plugin
go build -o ../bin/kubectl-documentdb
```

The compiled binary will live at `plugins/bin/kubectl-documentdb` relative to the repo root.

## Add the Plugin to Your PATH

1. Decide on an install directory (for example `$HOME/.local/bin`).
2. Copy the freshly built binary into that directory and make it executable:
   ```bash
   install -m 0755 plugins/bin/kubectl-documentdb $HOME/.local/bin/
   ```
3. Ensure the directory is on your PATH (add this to your shell profile if it isn’t already):
   ```bash
   export PATH="$HOME/.local/bin:$PATH"
   ```
4. Confirm the binary is discoverable:
   ```bash
   which kubectl-documentdb
   ```

> Kubectl treats any `kubectl-*` executable on the PATH as a plugin. Once the binary is on the PATH, the subcommand becomes available as `kubectl documentdb`.

## Sanity Test the CLI

With the binary on your PATH, run the built-in help to verify the plugin loads:

```bash
kubectl documentdb --help
```

You should see the usage banner rendered by `cmd/root.go`.

To smoke-test a real command (requires cluster access and contexts configured), run:

```bash
kubectl documentdb status --help
```

For clusters that have the DocumentDB CR deployed, you can list the current status:

```bash
kubectl documentdb status --context <member-cluster> --namespace <namespace>
```

Replace `<member-cluster>` and `<namespace>` with the appropriate values (e.g., `member-westus3-4hfp4a5ag24kg` and `documentdb-preview-ns`).

That’s all a new tester needs to build, install, and exercise the DocumentDB kubectl plugin.
