<!-- markdownlint-disable -->
## talosctl logs

Retrieve logs for a service

### Synopsis

Retrieve logs for a service

```
talosctl logs <service name> [flags]
```

### Options

```
  -f, --follow       specify if the logs should be streamed
  -h, --help         help for logs
  -k, --kubernetes   use the k8s.io containerd namespace
      --tail int32   lines of log file to display (default is to show from the beginning) (default -1)
  -c, --use-cri      use the CRI driver
```

### Options inherited from parent commands

```
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file (default "/home/user/.talos/config")
```

### SEE ALSO

* [talosctl](talosctl.md)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

