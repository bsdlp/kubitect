package terraform

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/MusicDin/kubitect/pkg/cluster/event"
	"github.com/MusicDin/kubitect/pkg/cluster/provisioner"
	"github.com/MusicDin/kubitect/pkg/env"
	"github.com/MusicDin/kubitect/pkg/models/config"
	"github.com/MusicDin/kubitect/pkg/ui"
	"github.com/MusicDin/kubitect/pkg/utils/cmp"
	"github.com/MusicDin/kubitect/pkg/utils/file"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/fs"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
)

type (
	terraform struct {
		// Required Terraform version
		version string

		// Path where terraform binary will be installed
		// if it is not found locally.
		binDir string

		// Evaluated during findAndInstall.
		binPath string

		// Dir where main.tf is located (root Terraform dir).
		projectDir string

		// If true, Terraform plan will be shown.
		showPlan bool

		// Configuration file containing values required for
		// main.tf template
		cfg *config.Config

		// Indicates that terraform project has
		// been already initialized.
		initialized bool
	}
)

func NewTerraformProvisioner(
	clusterPath,
	sharedPath string,
	showPlan bool,
	cfg *config.Config,
) provisioner.Provisioner {
	version := env.ConstTerraformVersion
	binDir := path.Join(sharedPath, "terraform", version)
	projDir := path.Join(clusterPath, "terraform")

	return &terraform{
		version:    version,
		binDir:     binDir,
		projectDir: projDir,
		showPlan:   showPlan,
		cfg:        cfg,
	}
}

// Init generates Terraform's main.tf file based on the provided cluster configuration.
func (t *terraform) Init(events event.Events) error {
	cfgPath := path.Join(t.projectDir, "variables.yaml")
	err := file.WriteYaml(t.cfg, cfgPath, 0644)
	if err != nil {
		return fmt.Errorf("terraform: failed to create input variables file: %v", err)
	}

	hosts := t.cfg.Hosts
	removedHosts := extractRemovedHosts(events)

	return NewMainTemplate(t.projectDir, hosts, removedHosts).Write()
}

// init initializes a Terraform project.
func (t *terraform) init() error {
	if t.initialized {
		return nil
	}

	binPath, err := t.findOrInstall()
	if err != nil {
		return err
	}

	t.binPath = binPath

	args := []string{
		flag("force-copy"),
		flag("input", false),
		flag("get", true),
		flag("upgrade"),
	}

	_, err = t.runCmd("init", args, true)

	if err == nil {
		t.initialized = true
	}

	return err
}

// Plan shows Terraform project changes (plan).
// It returns a potential error and whether there
// are changes or not.
func (t *terraform) Plan() (bool, error) {
	if err := t.init(); err != nil {
		return false, err
	}

	args := []string{
		flag("detailed-exitcode"),
		flag("input", false),
		flag("lock", true),
		flag("lock-timeout", "0s"),
		flag("parallelism", 10),
		flag("refresh", true),
	}

	exitCode, err := t.runCmd("plan", args, t.showPlan)

	// "exitCode 2" indicates terraform plan changes
	if err != nil && exitCode == 2 {
		return true, nil
	}

	return false, err
}

// Apply applies new Terraform configurations. In case any
// changes are detected, user confirmation is required.
func (t *terraform) Apply() error {
	changes, err := t.Plan()

	if err != nil {
		return err
	}

	// Ask user for permission if there are any changes
	if changes && t.showPlan {
		err := ui.Ask("Proceed with terraform apply?")

		if err != nil {
			return err
		}
	}

	args := []string{
		flag("auto-approve"),
		flag("input", false),
		flag("lock", true),
		flag("lock-timeout", "0s"),
		flag("parallelism", 10),
		flag("refresh", true),
	}

	_, err = t.runCmd("apply", args, true)
	return err
}

// Destroy destroys the Terraform project.
func (t *terraform) Destroy() error {
	err := t.init()
	if err != nil {
		return err
	}

	args := []string{
		flag("auto-approve"),
		flag("input", false),
		flag("lock", true),
		flag("lock-timeout", "0s"),
		flag("parallelism", 10),
		flag("refresh", true),
	}

	_, err = t.runCmd("destroy", args, true)

	return err
}

// findOrInstall first searches for Terraform binary locally and
// if binary is not found, it is installed in given binDir.
func (t *terraform) findOrInstall() (string, error) {
	var binPath string
	var err error

	ui.Printf(ui.INFO, "Ensuring Terraform %s is installed...\n", t.version)

	binPath, err = findTerraform(t.version, t.binDir)

	if err == nil {
		ui.Printf(ui.INFO, "Terraform %s found locally (%s).\n", t.version, binPath)
		return binPath, nil
	}

	ui.Printf(ui.INFO, "Terraform %s could not be found locally.\n", t.version)
	ui.Printf(ui.INFO, "Installing Terraform %s in '%s'...\n", t.version, t.binDir)

	binPath, err = installTerraform(t.version, t.binDir)

	if err != nil {
		return "", fmt.Errorf("failed to install Terraform: %v", err)
	}

	return binPath, nil
}

// findTerraform searches for Terraform binary locally.
// If binary is found, its path is returned.
func findTerraform(ver, binDir string) (string, error) {
	fs := &fs.ExactVersion{
		Product:    product.Terraform,
		Version:    version.Must(version.NewVersion(ver)),
		ExtraPaths: []string{binDir},
	}

	return fs.Find(context.Background())
}

// installTerraform installs Terraform binary of the provided
// version in a given directory.
func installTerraform(ver, binDir string) (string, error) {
	if err := os.MkdirAll(binDir, os.ModePerm); err != nil {
		return "", err
	}

	installer := &releases.ExactVersion{
		Product:    product.Terraform,
		Version:    version.Must(version.NewVersion(ver)),
		InstallDir: binDir,
	}

	return installer.Install(context.Background())
}

// Flag concatenates key and value with "=" if value is provided.
func flag(key string, value ...interface{}) string {
	if len(value) > 0 && value[0] != nil {
		return fmt.Sprintf("-%s=%v", key, value[0])
	}

	return fmt.Sprintf("-%s", key)
}

// extractRemovedHosts iterates over provided events and extracts
// hosts that have been removed.
func extractRemovedHosts(events event.Events) []config.Host {
	var hosts []config.Host
	for _, e := range events {
		if e.Action() != cmp.DELETE {
			continue
		}

		for _, ch := range e.Changes() {
			if host, ok := ch.Before.(config.Host); ok {
				hosts = append(hosts, host)
			}
		}
	}
	return hosts
}
