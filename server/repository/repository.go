package repository

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/text"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/common"
	repositorypkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/repository"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	servercache "github.com/argoproj/argo-cd/v3/server/cache"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/errors"
	"github.com/argoproj/argo-cd/v3/util/git"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/rbac"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// Server provides a Repository service
type Server struct {
	db              db.ArgoDB
	repoClientset   apiclient.Clientset
	enf             *rbac.Enforcer
	cache           *servercache.Cache
	appLister       applisters.ApplicationLister
	projLister      cache.SharedIndexInformer
	settings        *settings.SettingsManager
	namespace       string
	hydratorEnabled bool
}

// NewServer returns a new instance of the Repository service
func NewServer(
	repoClientset apiclient.Clientset,
	db db.ArgoDB,
	enf *rbac.Enforcer,
	cache *servercache.Cache,
	appLister applisters.ApplicationLister,
	projLister cache.SharedIndexInformer,
	namespace string,
	settings *settings.SettingsManager,
	hydratorEnabled bool,
) *Server {
	return &Server{
		db:              db,
		repoClientset:   repoClientset,
		enf:             enf,
		cache:           cache,
		appLister:       appLister,
		projLister:      projLister,
		namespace:       namespace,
		settings:        settings,
		hydratorEnabled: hydratorEnabled,
	}
}

func (s *Server) getRepo(ctx context.Context, url, project string) (*v1alpha1.Repository, error) {
	repo, err := s.db.GetRepository(ctx, url, project)
	if err != nil {
		return nil, common.PermissionDeniedAPIError
	}
	return repo, nil
}

func (s *Server) getWriteRepo(ctx context.Context, url, project string) (*v1alpha1.Repository, error) {
	repo, err := s.db.GetWriteRepository(ctx, url, project)
	if err != nil {
		return nil, common.PermissionDeniedAPIError
	}
	return repo, nil
}

func createRBACObject(project string, repo string) string {
	if project != "" {
		return project + "/" + repo
	}
	return repo
}

// Get the connection state for a given repository URL by connecting to the
// repo and evaluate the results. Unless forceRefresh is set to true, the
// result may be retrieved out of the cache.
func (s *Server) getConnectionState(ctx context.Context, url string, project string, forceRefresh bool) v1alpha1.ConnectionState {
	if !forceRefresh {
		if connectionState, err := s.cache.GetRepoConnectionState(url, project); err == nil {
			return connectionState
		}
	}
	now := metav1.Now()
	connectionState := v1alpha1.ConnectionState{
		Status:     v1alpha1.ConnectionStatusSuccessful,
		ModifiedAt: &now,
	}
	var err error
	repo, err := s.db.GetRepository(ctx, url, project)
	if err == nil {
		err = s.testRepo(ctx, repo)
	}
	if err != nil {
		connectionState.Status = v1alpha1.ConnectionStatusFailed
		if errors.IsCredentialsConfigurationError(err) {
			connectionState.Message = "Configuration error - please check the server logs"
			log.Warnf("could not retrieve repo: %s", err.Error())
		} else {
			connectionState.Message = fmt.Sprintf("Unable to connect to repository: %v", err)
		}
	}
	err = s.cache.SetRepoConnectionState(url, project, &connectionState)
	if err != nil {
		log.Warnf("getConnectionState cache set error %s: %v", url, err)
	}
	return connectionState
}

// List returns list of repositories
// Deprecated: Use ListRepositories instead
func (s *Server) List(ctx context.Context, q *repositorypkg.RepoQuery) (*v1alpha1.RepositoryList, error) {
	return s.ListRepositories(ctx, q)
}

// Get return the requested configured repository by URL and the state of its connections.
func (s *Server) Get(ctx context.Context, q *repositorypkg.RepoQuery) (*v1alpha1.Repository, error) {
	// ListRepositories normalizes the repo, sanitizes it, and augments it with connection details.
	repo, err := getRepository(ctx, s.ListRepositories, q)
	if err != nil {
		return nil, err
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceRepositories, rbac.ActionGet, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}

	exists, err := s.db.RepositoryExists(ctx, q.Repo, repo.Project)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, status.Errorf(codes.NotFound, "repo '%s' not found", q.Repo)
	}

	return repo, nil
}

func (s *Server) GetWrite(ctx context.Context, q *repositorypkg.RepoQuery) (*v1alpha1.Repository, error) {
	if !s.hydratorEnabled {
		return nil, status.Error(codes.Unimplemented, "hydrator is disabled")
	}

	repo, err := getRepository(ctx, s.ListWriteRepositories, q)
	if err != nil {
		return nil, err
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceWriteRepositories, rbac.ActionGet, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}

	exists, err := s.db.WriteRepositoryExists(ctx, q.Repo, repo.Project)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, status.Errorf(codes.NotFound, "write repo '%s' not found", q.Repo)
	}

	return repo, nil
}

// ListRepositories returns a list of all configured repositories and the state of their connections
func (s *Server) ListRepositories(ctx context.Context, q *repositorypkg.RepoQuery) (*v1alpha1.RepositoryList, error) {
	repos, err := s.db.ListRepositories(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.prepareRepoList(ctx, rbac.ResourceRepositories, repos, q.ForceRefresh)
	if err != nil {
		return nil, err
	}
	return &v1alpha1.RepositoryList{Items: items}, nil
}

// ListWriteRepositories returns a list of all configured repositories where the user has write access and the state of
// their connections
func (s *Server) ListWriteRepositories(ctx context.Context, q *repositorypkg.RepoQuery) (*v1alpha1.RepositoryList, error) {
	if !s.hydratorEnabled {
		return nil, status.Error(codes.Unimplemented, "hydrator is disabled")
	}

	repos, err := s.db.ListWriteRepositories(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.prepareRepoList(ctx, rbac.ResourceWriteRepositories, repos, q.ForceRefresh)
	if err != nil {
		return nil, err
	}
	return &v1alpha1.RepositoryList{Items: items}, nil
}

// ListRepositoriesByAppProject returns a list of all configured repositories and the state of their connections. It
// normalizes, sanitizes, and filters out repositories that the user does not have access to in the specified project.
// It also sorts the repositories by project and repo name.
func (s *Server) prepareRepoList(ctx context.Context, resourceType string, repos []*v1alpha1.Repository, forceRefresh bool) (v1alpha1.Repositories, error) {
	items := v1alpha1.Repositories{}
	for _, repo := range repos {
		items = append(items, repo.Normalize().Sanitized())
	}
	items = items.Filter(func(r *v1alpha1.Repository) bool {
		return s.enf.Enforce(ctx.Value("claims"), resourceType, rbac.ActionGet, createRBACObject(r.Project, r.Repo))
	})
	err := kube.RunAllAsync(len(items), func(i int) error {
		items[i].ConnectionState = s.getConnectionState(ctx, items[i].Repo, items[i].Project, forceRefresh)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		first := items[i]
		second := items[j]
		return fmt.Sprintf("%s/%s", first.Project, first.Repo) < fmt.Sprintf("%s/%s", second.Project, second.Repo)
	})
	return items, nil
}

func (s *Server) ListOCITags(ctx context.Context, q *repositorypkg.RepoQuery) (*apiclient.Refs, error) {
	repo, err := s.getRepo(ctx, q.Repo, q.GetAppProject())
	if err != nil {
		return nil, err
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceRepositories, rbac.ActionGet, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}

	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	defer utilio.Close(conn)

	return repoClient.ListOCITags(ctx, &apiclient.ListRefsRequest{
		Repo: repo,
	})
}

func (s *Server) ListRefs(ctx context.Context, q *repositorypkg.RepoQuery) (*apiclient.Refs, error) {
	repo, err := s.getRepo(ctx, q.Repo, q.GetAppProject())
	if err != nil {
		return nil, err
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceRepositories, rbac.ActionGet, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}

	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	defer utilio.Close(conn)

	return repoClient.ListRefs(ctx, &apiclient.ListRefsRequest{
		Repo: repo,
	})
}

// ListApps performs discovery of a git repository for potential sources of applications. Used
// as a convenience to the UI for auto-complete.
func (s *Server) ListApps(ctx context.Context, q *repositorypkg.RepoAppsQuery) (*repositorypkg.RepoAppsResponse, error) {
	repo, err := s.getRepo(ctx, q.Repo, q.GetAppProject())
	if err != nil {
		return nil, err
	}

	claims := ctx.Value("claims")
	if err := s.enf.EnforceErr(claims, rbac.ResourceRepositories, rbac.ActionGet, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}

	// This endpoint causes us to clone git repos & invoke config management tooling for the purposes
	// of app discovery. Only allow this to happen if user has privileges to create or update the
	// application which it wants to retrieve these details for.
	appRBACresource := fmt.Sprintf("%s/%s", q.AppProject, q.AppName)
	if !s.enf.Enforce(claims, rbac.ResourceApplications, rbac.ActionCreate, appRBACresource) &&
		!s.enf.Enforce(claims, rbac.ResourceApplications, rbac.ActionUpdate, appRBACresource) {
		return nil, common.PermissionDeniedAPIError
	}
	// Also ensure the repo is actually allowed in the project in question
	if err := s.isRepoPermittedInProject(ctx, q.Repo, q.AppProject); err != nil {
		return nil, err
	}

	// Test the repo
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	defer utilio.Close(conn)

	apps, err := repoClient.ListApps(ctx, &apiclient.ListAppsRequest{
		Repo:     repo,
		Revision: q.Revision,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*repositorypkg.AppInfo, 0)
	for app, appType := range apps.Apps {
		items = append(items, &repositorypkg.AppInfo{Path: app, Type: appType})
	}
	return &repositorypkg.RepoAppsResponse{Items: items}, nil
}

// GetAppDetails shows parameter values to various config tools (e.g. helm/kustomize values)
// This is used by UI for parameter form fields during app create & edit pages.
// It is also used when showing history of parameters used in previous syncs in the app history.
func (s *Server) GetAppDetails(ctx context.Context, q *repositorypkg.RepoAppDetailsQuery) (*apiclient.RepoAppDetailsResponse, error) {
	if q.Source == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing payload in request")
	}
	repo, err := s.getRepo(ctx, q.Source.RepoURL, q.GetAppProject())
	if err != nil {
		return nil, err
	}
	claims := ctx.Value("claims")
	if err := s.enf.EnforceErr(claims, rbac.ResourceRepositories, rbac.ActionGet, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}
	appName, appNs := argo.ParseFromQualifiedName(q.AppName, s.settings.GetNamespace())
	app, err := s.appLister.Applications(appNs).Get(appName)
	appRBACObj := createRBACObject(q.AppProject, q.AppName)
	// ensure caller has read privileges to app
	if err := s.enf.EnforceErr(claims, rbac.ResourceApplications, rbac.ActionGet, appRBACObj); err != nil {
		return nil, err
	}
	if apierrors.IsNotFound(err) {
		// app doesn't exist since it still is being formulated. verify they can create the app
		// before we reveal repo details
		if err := s.enf.EnforceErr(claims, rbac.ResourceApplications, rbac.ActionCreate, appRBACObj); err != nil {
			return nil, err
		}
	} else {
		// if we get here we are returning repo details of an existing app
		if q.AppProject != app.Spec.Project {
			return nil, common.PermissionDeniedAPIError
		}
		// verify caller is not making a request with arbitrary source values which were not in our history
		if !isSourceInHistory(app, *q.Source, q.SourceIndex, q.VersionId) {
			return nil, common.PermissionDeniedAPIError
		}
	}
	// Ensure the repo is actually allowed in the project in question
	if err := s.isRepoPermittedInProject(ctx, q.Source.RepoURL, q.AppProject); err != nil {
		return nil, err
	}

	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	defer utilio.Close(conn)
	helmRepos, err := s.db.ListHelmRepositories(ctx)
	if err != nil {
		return nil, err
	}
	kustomizeSettings, err := s.settings.GetKustomizeSettings()
	if err != nil {
		return nil, err
	}
	helmOptions, err := s.settings.GetHelmSettings()
	if err != nil {
		return nil, err
	}

	refSources := make(v1alpha1.RefTargetRevisionMapping)
	if app != nil && app.Spec.HasMultipleSources() {
		// Store the map of all sources having ref field into a map for applications with sources field
		refSources, err = argo.GetRefSources(ctx, app.Spec.Sources, q.AppProject, s.db.GetRepository, []string{})
		if err != nil {
			return nil, fmt.Errorf("failed to get ref sources: %w", err)
		}
	}

	return repoClient.GetAppDetails(ctx, &apiclient.RepoServerAppDetailsQuery{
		Repo:             repo,
		Source:           q.Source,
		Repos:            helmRepos,
		KustomizeOptions: kustomizeSettings,
		HelmOptions:      helmOptions,
		AppName:          q.AppName,
		RefSources:       refSources,
	})
}

// GetHelmCharts returns list of helm charts in the specified repository
func (s *Server) GetHelmCharts(ctx context.Context, q *repositorypkg.RepoQuery) (*apiclient.HelmChartsResponse, error) {
	repo, err := s.getRepo(ctx, q.Repo, q.GetAppProject())
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceRepositories, rbac.ActionGet, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	defer utilio.Close(conn)
	return repoClient.GetHelmCharts(ctx, &apiclient.HelmChartsRequest{Repo: repo})
}

// Create creates a repository or repository credential set
// Deprecated: Use CreateRepository() instead
func (s *Server) Create(ctx context.Context, q *repositorypkg.RepoCreateRequest) (*v1alpha1.Repository, error) {
	return s.CreateRepository(ctx, q)
}

// CreateRepository creates a repository configuration
func (s *Server) CreateRepository(ctx context.Context, q *repositorypkg.RepoCreateRequest) (*v1alpha1.Repository, error) {
	if q.Repo == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing payload in request")
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceRepositories, rbac.ActionCreate, createRBACObject(q.Repo.Project, q.Repo.Repo)); err != nil {
		return nil, err
	}

	var repo *v1alpha1.Repository
	var err error

	// check we can connect to the repo, copying any existing creds (not supported for project scoped repositories)
	if q.Repo.Project == "" {
		repo := q.Repo.DeepCopy()
		if !repo.HasCredentials() {
			creds, err := s.db.GetRepositoryCredentials(ctx, repo.Repo)
			if err != nil {
				return nil, err
			}
			repo.CopyCredentialsFrom(creds)
		}

		err = s.testRepo(ctx, repo)
		if err != nil {
			return nil, err
		}
	}

	r := q.Repo
	r.ConnectionState = v1alpha1.ConnectionState{Status: v1alpha1.ConnectionStatusSuccessful}
	repo, err = s.db.CreateRepository(ctx, r)
	if status.Convert(err).Code() == codes.AlreadyExists {
		// act idempotent if existing spec matches new spec
		existing, getErr := s.db.GetRepository(ctx, r.Repo, q.Repo.Project)
		if getErr != nil {
			return nil, status.Errorf(codes.Internal, "unable to check existing repository details: %v", getErr)
		}

		existing.Type = text.FirstNonEmpty(existing.Type, "git")
		// repository ConnectionState may differ, so make consistent before testing
		existing.ConnectionState = r.ConnectionState
		switch {
		case reflect.DeepEqual(existing, r):
			repo, err = existing, nil
		case q.Upsert:
			r.Project = q.Repo.Project
			return s.db.UpdateRepository(ctx, r)
		default:
			return nil, status.Error(codes.InvalidArgument, argo.GenerateSpecIsDifferentErrorMessage("repository", existing, r))
		}
	}
	if err != nil {
		return nil, err
	}
	return &v1alpha1.Repository{Repo: repo.Repo, Type: repo.Type, Name: repo.Name}, nil
}

// CreateWriteRepository creates a repository configuration with write credentials
func (s *Server) CreateWriteRepository(ctx context.Context, q *repositorypkg.RepoCreateRequest) (*v1alpha1.Repository, error) {
	if !s.hydratorEnabled {
		return nil, status.Error(codes.Unimplemented, "hydrator is disabled")
	}

	if q.Repo == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing payload in request")
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceWriteRepositories, rbac.ActionCreate, createRBACObject(q.Repo.Project, q.Repo.Repo)); err != nil {
		return nil, err
	}

	if !q.Repo.HasCredentials() {
		return nil, status.Errorf(codes.InvalidArgument, "missing credentials in request")
	}

	err := s.testRepo(ctx, q.Repo)
	if err != nil {
		return nil, err
	}

	repo, err := s.db.CreateWriteRepository(ctx, q.Repo)
	if status.Convert(err).Code() == codes.AlreadyExists {
		// act idempotent if existing spec matches new spec
		existing, getErr := s.db.GetWriteRepository(ctx, q.Repo.Repo, q.Repo.Project)
		if getErr != nil {
			return nil, status.Errorf(codes.Internal, "unable to check existing repository details: %v", getErr)
		}
		switch {
		case reflect.DeepEqual(existing, q.Repo):
			repo, err = existing, nil
		case q.Upsert:
			return s.db.UpdateWriteRepository(ctx, q.Repo)
		default:
			return nil, status.Error(codes.InvalidArgument, argo.GenerateSpecIsDifferentErrorMessage("write repository", existing, q.Repo))
		}
	}
	if err != nil {
		return nil, err
	}
	return &v1alpha1.Repository{Repo: repo.Repo, Type: repo.Type, Name: repo.Name}, nil
}

// Update updates a repository or credential set
// Deprecated: Use UpdateRepository() instead
func (s *Server) Update(ctx context.Context, q *repositorypkg.RepoUpdateRequest) (*v1alpha1.Repository, error) {
	return s.UpdateRepository(ctx, q)
}

// UpdateRepository updates a repository configuration
func (s *Server) UpdateRepository(ctx context.Context, q *repositorypkg.RepoUpdateRequest) (*v1alpha1.Repository, error) {
	if q.Repo == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing payload in request")
	}

	repo, err := s.getRepo(ctx, q.Repo.Repo, q.Repo.Project)
	if err != nil {
		return nil, err
	}

	// verify that user can do update inside project where repository is located
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceRepositories, rbac.ActionUpdate, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}
	// verify that user can do update inside project where repository will be located
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceRepositories, rbac.ActionUpdate, createRBACObject(q.Repo.Project, q.Repo.Repo)); err != nil {
		return nil, err
	}
	_, err = s.db.UpdateRepository(ctx, q.Repo)
	return &v1alpha1.Repository{Repo: q.Repo.Repo, Type: q.Repo.Type, Name: q.Repo.Name}, err
}

// UpdateWriteRepository updates a repository configuration with write credentials
func (s *Server) UpdateWriteRepository(ctx context.Context, q *repositorypkg.RepoUpdateRequest) (*v1alpha1.Repository, error) {
	if !s.hydratorEnabled {
		return nil, status.Error(codes.Unimplemented, "hydrator is disabled")
	}

	if q.Repo == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing payload in request")
	}

	repo, err := s.getWriteRepo(ctx, q.Repo.Repo, q.Repo.Project)
	if err != nil {
		return nil, err
	}

	// verify that user can do update inside project where repository is located
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceWriteRepositories, rbac.ActionUpdate, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}
	// verify that user can do update inside project where repository will be located
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceWriteRepositories, rbac.ActionUpdate, createRBACObject(q.Repo.Project, q.Repo.Repo)); err != nil {
		return nil, err
	}
	_, err = s.db.UpdateWriteRepository(ctx, q.Repo)
	return &v1alpha1.Repository{Repo: q.Repo.Repo, Type: q.Repo.Type, Name: q.Repo.Name}, err
}

// Delete removes a repository from the configuration
// Deprecated: Use DeleteRepository() instead
func (s *Server) Delete(ctx context.Context, q *repositorypkg.RepoQuery) (*repositorypkg.RepoResponse, error) {
	return s.DeleteRepository(ctx, q)
}

// DeleteRepository removes a repository from the configuration
func (s *Server) DeleteRepository(ctx context.Context, q *repositorypkg.RepoQuery) (*repositorypkg.RepoResponse, error) {
	repo, err := getRepository(ctx, s.ListRepositories, q)
	if err != nil {
		return nil, err
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceRepositories, rbac.ActionDelete, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}

	// invalidate cache
	if err := s.cache.SetRepoConnectionState(repo.Repo, repo.Project, nil); err != nil {
		log.Errorf("error invalidating cache: %v", err)
	}

	err = s.db.DeleteRepository(ctx, repo.Repo, repo.Project)
	return &repositorypkg.RepoResponse{}, err
}

// DeleteWriteRepository removes a repository from the configuration
func (s *Server) DeleteWriteRepository(ctx context.Context, q *repositorypkg.RepoQuery) (*repositorypkg.RepoResponse, error) {
	if !s.hydratorEnabled {
		return nil, status.Error(codes.Unimplemented, "hydrator is disabled")
	}

	repo, err := getRepository(ctx, s.ListWriteRepositories, q)
	if err != nil {
		return nil, err
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceWriteRepositories, rbac.ActionDelete, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}

	err = s.db.DeleteWriteRepository(ctx, repo.Repo, repo.Project)
	return &repositorypkg.RepoResponse{}, err
}

// getRepository fetches a single repository which the user has access to. If only one repository can be found which
// matches the same URL, that will be returned (this is for backward compatibility reasons). If multiple repositories
// are matched, a repository is only returned if it matches the app project of the incoming request.
func getRepository(ctx context.Context, listRepositories func(context.Context, *repositorypkg.RepoQuery) (*v1alpha1.RepositoryList, error), q *repositorypkg.RepoQuery) (*v1alpha1.Repository, error) {
	repositories, err := listRepositories(ctx, q)
	if err != nil {
		return nil, err
	}

	var foundRepos []*v1alpha1.Repository
	for _, repo := range repositories.Items {
		if git.SameURL(repo.Repo, q.Repo) {
			foundRepos = append(foundRepos, repo)
		}
	}

	if len(foundRepos) == 0 {
		return nil, common.PermissionDeniedAPIError
	}

	var foundRepo *v1alpha1.Repository
	if len(foundRepos) == 1 && q.GetAppProject() == "" {
		foundRepo = foundRepos[0]
	} else if len(foundRepos) > 0 {
		for _, repo := range foundRepos {
			if repo.Project == q.GetAppProject() {
				foundRepo = repo
				break
			}
		}
	}

	if foundRepo == nil {
		return nil, fmt.Errorf("repository not found for url %q and project %q", q.Repo, q.GetAppProject())
	}

	return foundRepo, nil
}

// ValidateAccess checks whether access to a repository is possible with the
// given URL and credentials.
func (s *Server) ValidateAccess(ctx context.Context, q *repositorypkg.RepoAccessQuery) (*repositorypkg.RepoResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceRepositories, rbac.ActionCreate, createRBACObject(q.Project, q.Repo)); err != nil {
		return nil, err
	}

	repo := &v1alpha1.Repository{
		Repo:                       q.Repo,
		Type:                       q.Type,
		Name:                       q.Name,
		Username:                   q.Username,
		Password:                   q.Password,
		BearerToken:                q.BearerToken,
		SSHPrivateKey:              q.SshPrivateKey,
		Insecure:                   q.Insecure,
		TLSClientCertData:          q.TlsClientCertData,
		TLSClientCertKey:           q.TlsClientCertKey,
		EnableOCI:                  q.EnableOci,
		GithubAppPrivateKey:        q.GithubAppPrivateKey,
		GithubAppId:                q.GithubAppID,
		GithubAppInstallationId:    q.GithubAppInstallationID,
		GitHubAppEnterpriseBaseURL: q.GithubAppEnterpriseBaseUrl,
		Proxy:                      q.Proxy,
		GCPServiceAccountKey:       q.GcpServiceAccountKey,
		InsecureOCIForceHttp:       q.InsecureOciForceHttp,
		UseAzureWorkloadIdentity:   q.UseAzureWorkloadIdentity,
	}

	// If repo does not have credentials, check if there are credentials stored
	// for it and if yes, copy them
	if !repo.HasCredentials() {
		repoCreds, err := s.db.GetRepositoryCredentials(ctx, q.Repo)
		if err != nil {
			return nil, err
		}
		if repoCreds != nil {
			repo.CopyCredentialsFrom(repoCreds)
		}
	}
	err := s.testRepo(ctx, repo)
	if err != nil {
		return nil, err
	}
	return &repositorypkg.RepoResponse{}, nil
}

// ValidateWriteAccess checks whether write access to a repository is possible with the
// given URL and credentials.
func (s *Server) ValidateWriteAccess(ctx context.Context, q *repositorypkg.RepoAccessQuery) (*repositorypkg.RepoResponse, error) {
	if !s.hydratorEnabled {
		return nil, status.Error(codes.Unimplemented, "hydrator is disabled")
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceWriteRepositories, rbac.ActionCreate, createRBACObject(q.Project, q.Repo)); err != nil {
		return nil, err
	}

	repo := &v1alpha1.Repository{
		Repo:                       q.Repo,
		Type:                       q.Type,
		Name:                       q.Name,
		Username:                   q.Username,
		Password:                   q.Password,
		BearerToken:                q.BearerToken,
		SSHPrivateKey:              q.SshPrivateKey,
		Insecure:                   q.Insecure,
		TLSClientCertData:          q.TlsClientCertData,
		TLSClientCertKey:           q.TlsClientCertKey,
		EnableOCI:                  q.EnableOci,
		GithubAppPrivateKey:        q.GithubAppPrivateKey,
		GithubAppId:                q.GithubAppID,
		GithubAppInstallationId:    q.GithubAppInstallationID,
		GitHubAppEnterpriseBaseURL: q.GithubAppEnterpriseBaseUrl,
		Proxy:                      q.Proxy,
		GCPServiceAccountKey:       q.GcpServiceAccountKey,
		UseAzureWorkloadIdentity:   q.UseAzureWorkloadIdentity,
	}

	err := s.testRepo(ctx, repo)
	if err != nil {
		return nil, err
	}
	return &repositorypkg.RepoResponse{}, nil
}

func (s *Server) testRepo(ctx context.Context, repo *v1alpha1.Repository) error {
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return fmt.Errorf("failed to connect to repo-server: %w", err)
	}
	defer utilio.Close(conn)

	_, err = repoClient.TestRepository(ctx, &apiclient.TestRepositoryRequest{
		Repo: repo,
	})
	return err
}

func (s *Server) isRepoPermittedInProject(ctx context.Context, repo string, projName string) error {
	proj, err := argo.GetAppProjectByName(ctx, projName, applisters.NewAppProjectLister(s.projLister.GetIndexer()), s.namespace, s.settings, s.db)
	if err != nil {
		return err
	}
	if !proj.IsSourcePermitted(v1alpha1.ApplicationSource{RepoURL: repo}) {
		return status.Errorf(codes.PermissionDenied, "repository '%s' not permitted in project '%s'", repo, projName)
	}
	return nil
}

// isSourceInHistory checks if the supplied application source is either our current application
// source, or was something which we synced to previously.
func isSourceInHistory(app *v1alpha1.Application, source v1alpha1.ApplicationSource, index int32, versionId int32) bool {
	// We have to check if the spec is within the source or sources split
	// and then iterate over the historical
	if app.Spec.HasMultipleSources() {
		appSources := app.Spec.GetSources()
		for _, s := range appSources {
			if source.Equals(&s) {
				return true
			}
		}
	} else {
		appSource := app.Spec.GetSource()
		if source.Equals(&appSource) {
			return true
		}
	}

	// Iterate history. When comparing items in our history, use the actual synced revision to
	// compare with the supplied source.targetRevision in the request. This is because
	// history[].source.targetRevision is ambiguous (e.g. HEAD), whereas
	// history[].revision will contain the explicit SHA
	// In case of multi source apps, we have to check the specific versionID because users
	// could have removed/added new sources and we cannot check all the versions due to that
	for _, h := range app.Status.History {
		// multi source revision
		if len(h.Sources) > 0 {
			if h.ID == int64(versionId) {
				if h.Revisions == nil {
					continue
				}
				h.Sources[index].TargetRevision = h.Revisions[index]
				if source.Equals(&h.Sources[index]) {
					return true
				}
			}
		} else { // single source revision
			h.Source.TargetRevision = h.Revision
			if source.Equals(&h.Source) {
				return true
			}
		}
	}

	return false
}
