package terraform

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/MusicDin/kubitect/pkg/models/config"
	"github.com/MusicDin/kubitect/pkg/utils/template"
)

type MainTemplate struct {
	Hosts        []config.Host
	RemovedHosts []config.Host
	projDir      string
}

func NewMainTemplate(projectDir string, hosts, removedHosts []config.Host) MainTemplate {
	return MainTemplate{
		Hosts:        hosts,
		RemovedHosts: removedHosts,
		projDir:      projectDir,
	}
}

func (t MainTemplate) Name() string {
	return "main.tf"
}

func (t MainTemplate) Functions() map[string]interface{} {
	return map[string]interface{}{
		"hostUri":     hostUri,
		"defaultHost": defaultHost,
	}
}

// Write creates main.tf file from template.
func (t MainTemplate) Write() error {
	srcPath := path.Join(t.projDir, "main.tf.tpl")
	dstPath := path.Join(t.projDir, "main.tf")

	return template.WriteFrom(t, srcPath, dstPath)
}

// defaultHost returns default host from a given list of hosts.
func defaultHost(hosts []config.Host) (config.Host, error) {
	if hosts == nil || len(hosts) == 0 {
		return config.Host{}, fmt.Errorf("defaultHost: hosts list is empty")
	}

	for _, h := range hosts {
		if h.Default {
			return h, nil
		}
	}

	return hosts[0], nil
}

// hostUri returns URI of a given host.
func hostUri(host config.Host) (string, error) {
	typ := host.Connection.Type

	if typ == "" || typ == config.LOCALHOST || typ == config.LOCAL {
		return "qemu:///system", nil
	}

	ip := string(host.Connection.IP)
	user := string(host.Connection.User)
	pkey := string(host.Connection.SSH.Keyfile)
	port := int(host.Connection.SSH.Port)
	verify := "&no_verify=1"

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	pkey = strings.Replace(pkey, "~", homeDir, 1)

	if host.Connection.SSH.Verify {
		verify = ""
	}

	uri := fmt.Sprintf("qemu+ssh://%s@%s:%d/system?keyfile=%s%s", user, ip, port, pkey, verify)
	return uri, nil
}
