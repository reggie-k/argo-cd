package app

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"

	rbacv1 "k8s.io/api/rbac/v1"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	client "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v3/util/grpc"
)

// this implements the "when" part of given/when/then
//
// none of the func implement error checks, and that is complete intended, you should check for errors
// using the Then()
type Actions struct {
	context      *Context
	lastOutput   string
	lastError    error
	ignoreErrors bool
}

func (a *Actions) IgnoreErrors() *Actions {
	a.ignoreErrors = true
	return a
}

func (a *Actions) DoNotIgnoreErrors() *Actions {
	a.ignoreErrors = false
	return a
}

func (a *Actions) PatchFile(file string, jsonPath string) *Actions {
	a.context.t.Helper()
	fixture.Patch(a.context.t, a.context.path+"/"+file, jsonPath)
	return a
}

func (a *Actions) DeleteFile(file string) *Actions {
	a.context.t.Helper()
	fixture.Delete(a.context.t, a.context.path+"/"+file)
	return a
}

func (a *Actions) WriteFile(fileName, fileContents string) *Actions {
	a.context.t.Helper()
	fixture.WriteFile(a.context.t, a.context.path+"/"+fileName, fileContents)
	return a
}

func (a *Actions) AddFile(fileName, fileContents string) *Actions {
	a.context.t.Helper()
	fixture.AddFile(a.context.t, a.context.path+"/"+fileName, fileContents)
	return a
}

func (a *Actions) AddSignedFile(fileName, fileContents string) *Actions {
	a.context.t.Helper()
	fixture.AddSignedFile(a.context.t, a.context.path+"/"+fileName, fileContents)
	return a
}

func (a *Actions) AddSignedTag(name string) *Actions {
	a.context.t.Helper()
	fixture.AddSignedTag(a.context.t, name)
	return a
}

func (a *Actions) AddTag(name string) *Actions {
	a.context.t.Helper()
	fixture.AddTag(a.context.t, name)
	return a
}

func (a *Actions) AddAnnotatedTag(name string, message string) *Actions {
	a.context.t.Helper()
	fixture.AddAnnotatedTag(a.context.t, name, message)
	return a
}

func (a *Actions) AddTagWithForce(name string) *Actions {
	a.context.t.Helper()
	fixture.AddTagWithForce(a.context.t, name)
	return a
}

func (a *Actions) RemoveSubmodule() *Actions {
	a.context.t.Helper()
	fixture.RemoveSubmodule(a.context.t)
	return a
}

func (a *Actions) CreateFromPartialFile(data string, flags ...string) *Actions {
	a.context.t.Helper()
	tmpFile, err := os.CreateTemp("", "")
	require.NoError(a.context.t, err)
	_, err = tmpFile.WriteString(data)
	require.NoError(a.context.t, err)

	args := append([]string{
		"app", "create",
		"-f", tmpFile.Name(),
		"--name", a.context.AppName(),
		"--repo", fixture.RepoURL(a.context.repoURLType),
		"--dest-server", a.context.destServer,
		"--dest-namespace", fixture.DeploymentNamespace(),
	}, flags...)
	if a.context.appNamespace != "" {
		args = append(args, "--app-namespace", a.context.appNamespace)
	}
	defer tmpFile.Close()
	a.runCli(args...)
	return a
}

func (a *Actions) CreateFromFile(handler func(app *v1alpha1.Application), flags ...string) *Actions {
	a.context.t.Helper()
	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.context.AppName(),
			Namespace: a.context.AppNamespace(),
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: a.context.project,
			Source: &v1alpha1.ApplicationSource{
				RepoURL: fixture.RepoURL(a.context.repoURLType),
				Path:    a.context.path,
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    a.context.destServer,
				Namespace: fixture.DeploymentNamespace(),
			},
		},
	}
	source := app.Spec.GetSource()
	if a.context.namePrefix != "" || a.context.nameSuffix != "" {
		source.Kustomize = &v1alpha1.ApplicationSourceKustomize{
			NamePrefix: a.context.namePrefix,
			NameSuffix: a.context.nameSuffix,
		}
	}
	if a.context.configManagementPlugin != "" {
		source.Plugin = &v1alpha1.ApplicationSourcePlugin{
			Name: a.context.configManagementPlugin,
		}
	}

	if len(a.context.parameters) > 0 {
		log.Fatal("v1alpha1.Application parameters or json tlas are not supported")
	}

	if a.context.directoryRecurse {
		source.Directory = &v1alpha1.ApplicationSourceDirectory{Recurse: true}
	}
	app.Spec.Source = &source

	handler(app)
	data := grpc.MustMarshal(app)
	tmpFile, err := os.CreateTemp("", "")
	require.NoError(a.context.t, err)
	_, err = tmpFile.Write(data)
	require.NoError(a.context.t, err)

	args := append([]string{
		"app", "create",
		"-f", tmpFile.Name(),
	}, flags...)
	defer tmpFile.Close()
	a.runCli(args...)
	return a
}

func (a *Actions) CreateMultiSourceAppFromFile(flags ...string) *Actions {
	a.context.t.Helper()
	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.context.AppName(),
			Namespace: a.context.AppNamespace(),
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: a.context.project,
			Sources: a.context.sources,
			Destination: v1alpha1.ApplicationDestination{
				Server:    a.context.destServer,
				Namespace: fixture.DeploymentNamespace(),
			},
			SyncPolicy: &v1alpha1.SyncPolicy{
				Automated: &v1alpha1.SyncPolicyAutomated{
					SelfHeal: true,
				},
			},
		},
	}

	data := grpc.MustMarshal(app)
	tmpFile, err := os.CreateTemp("", "")
	require.NoError(a.context.t, err)
	_, err = tmpFile.Write(data)
	require.NoError(a.context.t, err)

	args := append([]string{
		"app", "create",
		"-f", tmpFile.Name(),
	}, flags...)
	defer tmpFile.Close()
	a.runCli(args...)
	return a
}

func (a *Actions) CreateWithNoNameSpace(args ...string) *Actions {
	args = a.prepareCreateAppArgs(args)
	//  are you adding new context values? if you only use them for this func, then use args instead
	a.runCli(args...)
	return a
}

func (a *Actions) CreateApp(args ...string) *Actions {
	args = a.prepareCreateAppArgs(args)
	args = append(args, "--dest-namespace", fixture.DeploymentNamespace())

	//  are you adding new context values? if you only use them for this func, then use args instead
	a.runCli(args...)

	return a
}

func (a *Actions) prepareCreateAppArgs(args []string) []string {
	a.context.t.Helper()
	args = append([]string{
		"app", "create", a.context.AppQualifiedName(),
	}, args...)

	if a.context.drySourceRevision != "" || a.context.drySourcePath != "" || a.context.syncSourcePath != "" || a.context.syncSourceBranch != "" || a.context.hydrateToBranch != "" {
		args = append(args, "--dry-source-repo", fixture.RepoURL(a.context.repoURLType))
	} else {
		var repo string
		if a.context.ociRegistryPath != "" && a.context.repoURLType == fixture.RepoURLTypeOCI {
			repo = fmt.Sprintf("%s/%s", a.context.ociRegistry, a.context.ociRegistryPath)
		} else {
			repo = fixture.RepoURL(a.context.repoURLType)
		}
		args = append(args, "--repo", repo)
	}

	if a.context.destName != "" && a.context.isDestServerInferred && !slices.Contains(args, "--dest-server") {
		args = append(args, "--dest-name", a.context.destName)
	} else {
		args = append(args, "--dest-server", a.context.destServer)
	}
	if a.context.path != "" {
		args = append(args, "--path", a.context.path)
	}

	if a.context.drySourceRevision != "" {
		args = append(args, "--dry-source-revision", a.context.drySourceRevision)
	}

	if a.context.drySourcePath != "" {
		args = append(args, "--dry-source-path", a.context.drySourcePath)
	}

	if a.context.syncSourceBranch != "" {
		args = append(args, "--sync-source-branch", a.context.syncSourceBranch)
	}

	if a.context.syncSourcePath != "" {
		args = append(args, "--sync-source-path", a.context.syncSourcePath)
	}

	if a.context.hydrateToBranch != "" {
		args = append(args, "--hydrate-to-branch", a.context.hydrateToBranch)
	}

	if a.context.chart != "" {
		args = append(args, "--helm-chart", a.context.chart)
	}

	if a.context.env != "" {
		args = append(args, "--env", a.context.env)
	}

	for _, parameter := range a.context.parameters {
		args = append(args, "--parameter", parameter)
	}

	args = append(args, "--project", a.context.project)

	if a.context.namePrefix != "" {
		args = append(args, "--nameprefix", a.context.namePrefix)
	}

	if a.context.nameSuffix != "" {
		args = append(args, "--namesuffix", a.context.nameSuffix)
	}

	if a.context.configManagementPlugin != "" {
		args = append(args, "--config-management-plugin", a.context.configManagementPlugin)
	}

	if a.context.revision != "" {
		args = append(args, "--revision", a.context.revision)
	}
	if a.context.helmPassCredentials {
		args = append(args, "--helm-pass-credentials")
	}
	if a.context.helmSkipCrds {
		args = append(args, "--helm-skip-crds")
	}
	if a.context.helmSkipSchemaValidation {
		args = append(args, "--helm-skip-schema-validation")
	}
	if a.context.helmSkipTests {
		args = append(args, "--helm-skip-tests")
	}
	return args
}

func (a *Actions) Declarative(filename string) *Actions {
	a.context.t.Helper()
	return a.DeclarativeWithCustomRepo(filename, fixture.RepoURL(a.context.repoURLType))
}

func (a *Actions) DeclarativeWithCustomRepo(filename string, repoURL string) *Actions {
	a.context.t.Helper()
	values := map[string]any{
		"ArgoCDNamespace":     fixture.TestNamespace(),
		"DeploymentNamespace": fixture.DeploymentNamespace(),
		"Name":                a.context.AppName(),
		"Path":                a.context.path,
		"Project":             a.context.project,
		"RepoURL":             repoURL,
	}
	a.lastOutput, a.lastError = fixture.Declarative(a.context.t, filename, values)
	a.verifyAction()
	return a
}

func (a *Actions) PatchApp(patch string) *Actions {
	a.context.t.Helper()
	a.runCli("app", "patch", a.context.AppQualifiedName(), "--patch", patch)
	return a
}

func (a *Actions) PatchAppHttp(patch string) *Actions { //nolint:revive //FIXME(var-naming)
	a.context.t.Helper()
	var application v1alpha1.Application
	patchType := "merge"
	appName := a.context.AppQualifiedName()
	appNamespace := a.context.AppNamespace()
	patchRequest := &client.ApplicationPatchRequest{
		Name:         &appName,
		PatchType:    &patchType,
		Patch:        &patch,
		AppNamespace: &appNamespace,
	}
	jsonBytes, err := json.MarshalIndent(patchRequest, "", "  ")
	require.NoError(a.context.t, err)
	err = fixture.DoHttpJsonRequest("PATCH",
		fmt.Sprintf("/api/v1/applications/%v", appName),
		&application,
		jsonBytes...)
	require.NoError(a.context.t, err)
	return a
}

func (a *Actions) AppSet(flags ...string) *Actions {
	a.context.t.Helper()
	args := []string{"app", "set", a.context.AppQualifiedName()}
	args = append(args, flags...)
	a.runCli(args...)
	return a
}

func (a *Actions) AppUnSet(flags ...string) *Actions {
	a.context.t.Helper()
	args := []string{"app", "unset", a.context.AppQualifiedName()}
	args = append(args, flags...)
	a.runCli(args...)
	return a
}

func (a *Actions) Sync(args ...string) *Actions {
	a.context.t.Helper()
	args = append([]string{"app", "sync"}, args...)
	if a.context.name != "" {
		args = append(args, a.context.AppQualifiedName())
	}
	args = append(args, "--timeout", strconv.Itoa(a.context.timeout))

	if a.context.async {
		args = append(args, "--async")
	}

	if a.context.prune {
		args = append(args, "--prune")
	}

	if a.context.resource != "" {
		// Waiting for the app to be successfully created.
		// Else the sync would fail to retrieve the app resources.
		a.context.Sleep(5)
		args = append(args, "--resource", a.context.resource)
	}

	if a.context.localPath != "" {
		args = append(args, "--local", a.context.localPath)
	}

	if a.context.force {
		args = append(args, "--force")
	}

	if a.context.applyOutOfSyncOnly {
		args = append(args, "--apply-out-of-sync-only")
	}

	if a.context.replace {
		args = append(args, "--replace")
	}

	//  are you adding new context values? if you only use them for this func, then use args instead

	a.runCli(args...)

	return a
}

func (a *Actions) ConfirmDeletion() *Actions {
	a.context.t.Helper()

	a.runCli("app", "confirm-deletion", a.context.AppQualifiedName())

	return a
}

func (a *Actions) TerminateOp() *Actions {
	a.context.t.Helper()
	a.runCli("app", "terminate-op", a.context.AppQualifiedName())
	return a
}

func (a *Actions) Refresh(refreshType v1alpha1.RefreshType) *Actions {
	a.context.t.Helper()
	flag := map[v1alpha1.RefreshType]string{
		v1alpha1.RefreshTypeNormal: "--refresh",
		v1alpha1.RefreshTypeHard:   "--hard-refresh",
	}[refreshType]

	a.runCli("app", "get", a.context.AppQualifiedName(), flag)

	return a
}

func (a *Actions) Get() *Actions {
	a.context.t.Helper()
	a.runCli("app", "get", a.context.AppQualifiedName())
	return a
}

func (a *Actions) Delete(cascade bool) *Actions {
	a.context.t.Helper()
	a.runCli("app", "delete", a.context.AppQualifiedName(), fmt.Sprintf("--cascade=%v", cascade), "--yes")
	return a
}

func (a *Actions) DeleteBySelector(selector string) *Actions {
	a.context.t.Helper()
	a.runCli("app", "delete", "--selector="+selector, "--yes")
	return a
}

func (a *Actions) DeleteBySelectorWithWait(selector string) *Actions {
	a.context.t.Helper()
	a.runCli("app", "delete", "--selector="+selector, "--yes", "--wait")
	return a
}

func (a *Actions) Wait(args ...string) *Actions {
	a.context.t.Helper()
	args = append([]string{"app", "wait"}, args...)
	if a.context.name != "" {
		args = append(args, a.context.AppQualifiedName())
	}
	args = append(args, "--timeout", strconv.Itoa(a.context.timeout))
	a.runCli(args...)
	return a
}

func (a *Actions) SetParamInSettingConfigMap(key, value string) *Actions {
	a.context.t.Helper()
	require.NoError(a.context.t, fixture.SetParamInSettingConfigMap(key, value))
	return a
}

func (a *Actions) And(block func()) *Actions {
	a.context.t.Helper()
	block()
	return a
}

func (a *Actions) Then() *Consequences {
	a.context.t.Helper()
	return &Consequences{a.context, a, 15}
}

func (a *Actions) runCli(args ...string) {
	a.context.t.Helper()
	a.lastOutput, a.lastError = fixture.RunCli(args...)
	a.verifyAction()
}

func (a *Actions) verifyAction() {
	a.context.t.Helper()
	if !a.ignoreErrors {
		a.Then().Expect(Success(""))
	}
}

func (a *Actions) SetTrackingMethod(trackingMethod string) *Actions {
	a.context.t.Helper()
	require.NoError(a.context.t, fixture.SetTrackingMethod(trackingMethod))
	return a
}

func (a *Actions) SetInstallationID(installationID string) *Actions {
	a.context.t.Helper()
	require.NoError(a.context.t, fixture.SetInstallationID(installationID))
	return a
}

func (a *Actions) SetTrackingLabel(trackingLabel string) *Actions {
	a.context.t.Helper()
	require.NoError(a.context.t, fixture.SetTrackingLabel(trackingLabel))
	return a
}

func (a *Actions) WithImpersonationEnabled(serviceAccountName string, policyRules []rbacv1.PolicyRule) *Actions {
	a.context.t.Helper()
	require.NoError(a.context.t, fixture.SetImpersonationEnabled("true"))
	if serviceAccountName == "" || policyRules == nil {
		return a
	}
	require.NoError(a.context.t, fixture.CreateRBACResourcesForImpersonation(serviceAccountName, policyRules))
	return a
}

func (a *Actions) WithImpersonationDisabled() *Actions {
	a.context.t.Helper()
	require.NoError(a.context.t, fixture.SetImpersonationEnabled("false"))
	return a
}
