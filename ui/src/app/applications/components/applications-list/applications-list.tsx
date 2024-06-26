import {Autocomplete, ErrorNotification, MockupList, NotificationType, SlidingPanel, Toolbar, Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {Key, KeybindingContext, KeybindingProvider} from 'argo-ui/v2';
import {RouteComponentProps} from 'react-router';
import {combineLatest, from, merge, Observable} from 'rxjs';
import {bufferTime, delay, filter, map, mergeMap, repeat, retryWhen} from 'rxjs/operators';
import {AddAuthToToolbar, ClusterCtx, DataLoader, EmptyState, ObservableQuery, Page, Paginate, Query, Spinner} from '../../../shared/components';
import {AuthSettingsCtx, Consumer, Context, ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {
    AppsListViewKey,
    AppsListPreferences,
    AppsListViewType,
    HealthStatusBarPreferences,
    services,
    AbstractAppsListPreferences,
    AppSetsListPreferences
} from '../../../shared/services';
import {ApplicationCreatePanel} from '../application-create-panel/application-create-panel';
import {ApplicationSyncPanel} from '../application-sync-panel/application-sync-panel';
import {ApplicationsSyncPanel} from '../applications-sync-panel/applications-sync-panel';
import * as AppUtils from '../utils';
import {AbstractFilteredApp, ApplicationsFilter, getFilterResults} from './applications-filter';
import {ApplicationsStatusBar} from './applications-status-bar';
import {ApplicationsSummary} from './applications-summary';
import {ApplicationsTable} from './applications-table';
import {AbstractApplicationTilesProps, ApplicationSetTilesProps, ApplicationTiles, ApplicationTilesProps} from './applications-tiles';
import {ApplicationsRefreshPanel} from '../applications-refresh-panel/applications-refresh-panel';
import {useSidebarTarget} from '../../../sidebar/sidebar';

import './applications-list.scss';
import './flex-top-bar.scss';
import {AbstractApplication, Application, ApplicationSet} from '../../../shared/models';
import {History} from 'history';

const EVENTS_BUFFER_TIMEOUT = 500;
const WATCH_RETRY_TIMEOUT = 500;

// The applications list/watch API supports only selected set of fields.
// Make sure to register any new fields in the `appFields` map of `pkg/apiclient/application/forwarder_overwrite.go`.
const APP_FIELDS = [
    'metadata.name',
    'metadata.namespace',
    'metadata.annotations',
    'metadata.labels',
    'metadata.creationTimestamp',
    'metadata.deletionTimestamp',
    'spec',
    'operation.sync',
    'status.sync.status',
    'status.sync.revision',
    'status.health',
    'status.operationState.phase',
    'status.operationState.finishedAt',
    'status.operationState.operation.sync',
    'status.summary',
    'status.resources'
];

const APPSET_FIELDS = ['metadata.name', 'metadata.namespace', 'metadata.annotations', 'metadata.labels', 'metadata.creationTimestamp', 'metadata.deletionTimestamp', 'spec'];

function getAppListFields(isFromApps: boolean): string[] {
    const APP_LIST_FIELDS = isFromApps
        ? ['metadata.resourceVersion', ...APP_FIELDS.map(field => `items.${field}`)]
        : ['metadata.resourceVersion', ...APPSET_FIELDS.map(field => `items.${field}`)];
    return APP_LIST_FIELDS;
}

function getAppWatchFields(isFromApps: boolean): string[] {
    const APP_WATCH_FIELDS = isFromApps
        ? ['result.type', ...APP_FIELDS.map(field => `result.application.${field}`)]
        : ['result.type', ...APPSET_FIELDS.map(field => `result.application.${field}`)];
    return APP_WATCH_FIELDS;
}

function loadApplications(
    ctx: ContextApis & {
        history: History<unknown>;
    },
    projects: string[],
    appNamespace: string,
    objectListKind: string
): Observable<models.AbstractApplication[]> {
    const isListOfApplications = objectListKind === 'application';
    return from(services.applications.list(projects, ctx, {appNamespace, fields: getAppListFields(isListOfApplications)})).pipe(
        mergeMap(applicationsList => {
            const applications = applicationsList.items;
            return merge(
                from([applications]),
                services.applications
                    .watch(ctx.history.location.pathname, {projects, resourceVersion: applicationsList.metadata.resourceVersion}, {fields: getAppWatchFields(isListOfApplications)})
                    .pipe(repeat())
                    .pipe(retryWhen(errors => errors.pipe(delay(WATCH_RETRY_TIMEOUT))))
                    // batch events to avoid constant re-rendering and improve UI performance
                    .pipe(bufferTime(EVENTS_BUFFER_TIMEOUT))
                    .pipe(
                        map(appChanges => {
                            appChanges.forEach(appChange => {
                                const index = applications.findIndex(item => AppUtils.appInstanceName(item) === AppUtils.appInstanceName(appChange.application));

                                switch (appChange.type) {
                                    case 'DELETED':
                                        if (index > -1) {
                                            applications.splice(index, 1);
                                        }
                                        break;
                                    default:
                                        if (index > -1) {
                                            applications[index] = appChange.application;
                                        } else {
                                            applications.unshift(appChange.application);
                                        }
                                        break;
                                }
                            });
                            return {applications, updated: appChanges.length > 0};
                        })
                    )
                    .pipe(filter(item => item.updated))
                    .pipe(map(item => item.applications))
                // .pipe(map(item => (isApp(applications[0]) ? (item.applications as models.Application[]) : (item.applications as models.ApplicationSet[]))))
            );
        })
    );
}

const ViewPref = ({children, objectListKind}: {children: (pref: AbstractAppsListPreferences & {page: number; search: string}) => React.ReactNode; objectListKind: string}) => {
    return (
        <ObservableQuery>
            {q => (
                <DataLoader
                    load={() =>
                        combineLatest([services.viewPreferences.getPreferences().pipe(map(item => item.appList)), q]).pipe(
                            map(items => {
                                const params = items[1];
                                const viewPref: AbstractAppsListPreferences = {...items[0]};
                                if (objectListKind === 'application') {
                                    // App specific filters
                                    if (params.get('proj') != null) {
                                        (viewPref as AppsListPreferences).projectsFilter = params
                                            .get('proj')
                                            .split(',')
                                            .filter(item => !!item);
                                    }
                                    if (params.get('sync') != null) {
                                        (viewPref as AppsListPreferences).syncFilter = params
                                            .get('sync')
                                            .split(',')
                                            .filter(item => !!item);
                                    }
                                    if (params.get('autoSync') != null) {
                                        (viewPref as AppsListPreferences).autoSyncFilter = params
                                            .get('autoSync')
                                            .split(',')
                                            .filter(item => !!item);
                                    }
                                    if (params.get('cluster') != null) {
                                        (viewPref as AppsListPreferences).clustersFilter = params
                                            .get('cluster')
                                            .split(',')
                                            .filter(item => !!item);
                                    }
                                    if (params.get('namespace') != null) {
                                        (viewPref as AppsListPreferences).namespacesFilter = params
                                            .get('namespace')
                                            .split(',')
                                            .filter(item => !!item);
                                    }
                                }
                                // App and AppSet common filters
                                if (params.get('health') != null) {
                                    viewPref.healthFilter = params
                                        .get('health')
                                        .split(',')
                                        .filter(item => !!item);
                                }

                                if (params.get('showFavorites') != null) {
                                    viewPref.showFavorites = params.get('showFavorites') === 'true';
                                }
                                if (params.get('view') != null) {
                                    viewPref.view = params.get('view') as AppsListViewType;
                                }
                                if (params.get('labels') != null) {
                                    viewPref.labelsFilter = params
                                        .get('labels')
                                        .split(',')
                                        .map(decodeURIComponent)
                                        .filter(item => !!item);
                                }
                                return {...viewPref, page: parseInt(params.get('page') || '0', 10), search: params.get('search') || ''};
                            })
                        )
                    }>
                    {pref => children(pref)}
                </DataLoader>
            )}
        </ObservableQuery>
    );
};

function filterApps(
    applications: AbstractApplication[],
    pref: AbstractAppsListPreferences,
    search: string,
    isListOfApplications: boolean
): {filteredApps: AbstractApplication[]; filterResults: AbstractFilteredApp[]} {
    applications = applications.map(app => {
        let isAppOfAppsPattern = false;
        if (!isListOfApplications) {
            // AppSet behaves like an app of apps
            isAppOfAppsPattern = true;
        } else {
            // It is an App and may or may not be app-of-apps pattern
            for (const resource of (app as models.Application).status.resources) {
                if (resource.kind === 'Application') {
                    isAppOfAppsPattern = true;
                    break;
                }
            }
        }
        return {...app, isAppOfAppsPattern};
    });
    const filterResults =
        applications.length === 0
            ? getFilterResults(applications, pref)
            : isListOfApplications
            ? getFilterResults(applications as Application[], pref as AppsListPreferences)
            : getFilterResults(applications as ApplicationSet[], pref as AppSetsListPreferences);
    return {
        filterResults,
        filteredApps: filterResults.filter(
            app => (search === '' || app.metadata.name.includes(search) || app.metadata.namespace.includes(search)) && Object.values(app.filterResult).every(val => val)
        )
    };
}

function tryJsonParse(input: string) {
    try {
        return (input && JSON.parse(input)) || null;
    } catch {
        return null;
    }
}

const SearchBar = (props: {
    content: string;
    ctx: ContextApis & {
        history: History<unknown>;
    };
    apps: models.AbstractApplication[];
    objectListKind: string;
}) => {
    const {content, ctx, apps} = {...props};

    const searchBar = React.useRef<HTMLDivElement>(null);

    const query = new URLSearchParams(window.location.search);
    const appInput = tryJsonParse(query.get('new'));

    const {useKeybinding} = React.useContext(KeybindingContext);
    const [isFocused, setFocus] = React.useState(false);
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);

    const placeholderText = props.objectListKind === 'application' ? 'Search applications...' : 'Search application sets...';

    useKeybinding({
        keys: Key.SLASH,
        action: () => {
            if (searchBar.current && !appInput) {
                searchBar.current.querySelector('input').focus();
                setFocus(true);
                return true;
            }
            return false;
        }
    });

    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            if (searchBar.current && !appInput && isFocused) {
                searchBar.current.querySelector('input').blur();
                setFocus(false);
                return true;
            }
            return false;
        }
    });

    return (
        <Autocomplete
            filterSuggestions={true}
            renderInput={inputProps => (
                <div className='applications-list__search' ref={searchBar}>
                    <i
                        className='fa fa-search'
                        style={{marginRight: '9px', cursor: 'pointer'}}
                        onClick={() => {
                            if (searchBar.current) {
                                searchBar.current.querySelector('input').focus();
                            }
                        }}
                    />
                    <input
                        {...inputProps}
                        onFocus={e => {
                            e.target.select();
                            if (inputProps.onFocus) {
                                inputProps.onFocus(e);
                            }
                        }}
                        style={{fontSize: '14px'}}
                        className='argo-field'
                        placeholder={placeholderText}
                    />
                    <div className='keyboard-hint'>/</div>
                    {content && (
                        <i className='fa fa-times' onClick={() => ctx.navigation.goto('.', {search: null}, {replace: true})} style={{cursor: 'pointer', marginLeft: '5px'}} />
                    )}
                </div>
            )}
            wrapperProps={{className: 'applications-list__search-wrapper'}}
            renderItem={item => (
                <React.Fragment>
                    <i className='icon argo-icon-application' /> {item.label}
                </React.Fragment>
            )}
            onSelect={val => {
                ctx.navigation.goto(`./${val}`);
            }}
            onChange={e => ctx.navigation.goto('.', {search: e.target.value}, {replace: true})}
            value={content || ''}
            items={apps.map(app => AppUtils.appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled))}
        />
    );
};

const FlexTopBar = (props: {toolbar: Toolbar | Observable<Toolbar>}) => {
    const ctx = React.useContext(Context);
    const loadToolbar = AddAuthToToolbar(props.toolbar, ctx);
    return (
        <React.Fragment>
            <div className='top-bar row flex-top-bar' key='tool-bar'>
                <DataLoader load={() => loadToolbar}>
                    {toolbar => (
                        <React.Fragment>
                            <div className='flex-top-bar__actions'>
                                {toolbar.actionMenu && (
                                    <React.Fragment>
                                        {toolbar.actionMenu.items.map((item, i) => (
                                            <button
                                                disabled={!!item.disabled}
                                                qe-id={item.qeId}
                                                className='argo-button argo-button--base'
                                                onClick={() => item.action()}
                                                style={{marginRight: 2}}
                                                key={i}>
                                                {item.iconClassName && <i className={item.iconClassName} style={{marginLeft: '-5px', marginRight: '5px'}} />}
                                                <span className='show-for-large'>{item.title}</span>
                                            </button>
                                        ))}
                                    </React.Fragment>
                                )}
                            </div>
                            <div className='flex-top-bar__tools'>{toolbar.tools}</div>
                        </React.Fragment>
                    )}
                </DataLoader>
            </div>
            <div className='flex-top-bar__padder' />
        </React.Fragment>
    );
};

interface RouteComponentPropsExtended extends RouteComponentProps {
    objectListKind: string;
}

export const ApplicationsList = (props: RouteComponentPropsExtended) => {
    const query = new URLSearchParams(props.location.search);
    const appInput = tryJsonParse(query.get('new'));
    const syncAppsInput = tryJsonParse(query.get('syncApps'));
    const refreshAppsInput = tryJsonParse(query.get('refreshApps'));
    const [createApi, setCreateApi] = React.useState(null);
    const clusters = React.useMemo(() => services.clusters.list(), []);
    const [isAppCreatePending, setAppCreatePending] = React.useState(false);
    const loaderRef = React.useRef<DataLoader>();
    const {List, Summary, Tiles} = AppsListViewKey;

    const listCtx = React.useContext(Context);

    const objectListKind = props.objectListKind;
    const isListOfApplications = objectListKind === 'application';

    function refreshApp(appName: string, appNamespace: string) {
        // app refreshing might be done too quickly so that UI might miss it due to event batching
        // add refreshing annotation in the UI to improve user experience
        if (loaderRef.current) {
            const applications = loaderRef.current.getData() as models.Application[];
            const app = applications.find(item => item.metadata.name === appName && item.metadata.namespace === appNamespace);
            if (app) {
                AppUtils.setAppRefreshing(app);
                loaderRef.current.setData(applications);
            }
        }
        services.applications.get(appName, appNamespace, 'normal');
    }

    function onFilterPrefChanged(newPref: AbstractAppsListPreferences) {
        services.viewPreferences.updatePreferences({appList: newPref});
        listCtx.navigation.goto(
            '.',

            {
                proj: isListOfApplications ? (newPref as AppsListPreferences).projectsFilter.join(',') : '',
                sync: isListOfApplications ? (newPref as AppsListPreferences).syncFilter.join(',') : '',
                autoSync: isListOfApplications ? (newPref as AppsListPreferences).autoSyncFilter.join(',') : '',
                health: newPref.healthFilter.join(','),
                namespace: isListOfApplications ? (newPref as AppsListPreferences).namespacesFilter.join(',') : '',
                cluster: isListOfApplications ? (newPref as AppsListPreferences).clustersFilter.join(',') : '',
                labels: newPref.labelsFilter.map(encodeURIComponent).join(',')
            },
            {replace: true}
        );
    }

    const pageTitlePrefix = isListOfApplications ? 'Applications ' : 'ApplicationSets ';

    function getPageTitle(view: string) {
        switch (view) {
            case List:
                return pageTitlePrefix + 'List';
            case Tiles:
                return pageTitlePrefix + 'Tiles';
            case Summary:
                return pageTitlePrefix + 'Summary';
        }
        return '';
    }

    const sidebarTarget = useSidebarTarget();

    const getEmptyStateText = isListOfApplications ? 'No matching applications found' : 'No matching application sets found';

    const applicationTilesProps = (data: models.Application[]): ApplicationTilesProps => {
        return {
            applications: data,
            syncApplication: (appName, appNamespace) => listCtx.navigation.goto('.', {syncApp: appName, appNamespace}, {replace: true}),

            refreshApplication: refreshApp,
            deleteApplication: (appName, appNamespace) => AppUtils.deleteApplication(appName, appNamespace, listCtx),
            objectListKind
        };
    };

    const applicationSetTilesProps = (data: models.ApplicationSet[]): ApplicationSetTilesProps => {
        return {
            applications: data,
            deleteApplication: (appName, appNamespace) => AppUtils.deleteApplication(appName, appNamespace, listCtx),
            objectListKind
        };
    };

    const abstractApplicationTilesProps = (applications: models.AbstractApplication[]): AbstractApplicationTilesProps => {
        if (isListOfApplications) {
            return applicationTilesProps(applications);
        } else {
            return applicationSetTilesProps(applications);
        }
    };

    function getProjectsFilter(pref: AbstractAppsListPreferences): string[] {
        return isListOfApplications ? (pref as AppsListPreferences & {page: number; search: string}).projectsFilter : [];
    }

    return (
        <ClusterCtx.Provider value={clusters}>
            <KeybindingProvider>
                <Consumer>
                    {ctx => (
                        <ViewPref objectListKind={objectListKind}>
                            {pref => (
                                <Page
                                    key={pref.view}
                                    title={getPageTitle(pref.view)}
                                    useTitleOnly={true}
                                    toolbar={
                                        isListOfApplications
                                            ? {breadcrumbs: [{title: 'Applications', path: '/applications'}]}
                                            : {breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'ApplicationSets'}]}
                                    }
                                    hideAuth={true}>
                                    <DataLoader
                                        input={isListOfApplications ? (pref as AppsListPreferences & {page: number; search: string}).projectsFilter?.join(',') : ''}
                                        ref={loaderRef}
                                        load={() =>
                                            AppUtils.handlePageVisibility(() =>
                                                loadApplications(
                                                    ctx,
                                                    isListOfApplications ? (pref as AppsListPreferences & {page: number; search: string}).projectsFilter : [],
                                                    query.get('appNamespace'),
                                                    objectListKind
                                                )
                                            )
                                        }
                                        loadingRenderer={() => (
                                            <div className='argo-container'>
                                                <MockupList height={100} marginTop={30} />
                                            </div>
                                        )}>
                                        {(applications: models.AbstractApplication[]) => {
                                            const healthBarPrefs = pref.statusBarView || ({} as HealthStatusBarPreferences);
                                            const {filteredApps, filterResults} = filterApps(
                                                isListOfApplications ? (applications as Application[]) : (applications as ApplicationSet[]),
                                                isListOfApplications
                                                    ? (pref as AppsListPreferences & {page: number; search: string})
                                                    : (pref as AppSetsListPreferences & {page: number; search: string}),
                                                pref.search,
                                                isListOfApplications
                                            );
                                            return (
                                                <React.Fragment>
                                                    <FlexTopBar
                                                        toolbar={{
                                                            tools: (
                                                                <React.Fragment key='app-list-tools'>
                                                                    <Query>
                                                                        {q => <SearchBar content={q.get('search')} apps={applications} ctx={ctx} objectListKind={objectListKind} />}
                                                                    </Query>
                                                                    <Tooltip content='Toggle Health Status Bar'>
                                                                        <button
                                                                            className={`applications-list__accordion argo-button argo-button--base${
                                                                                healthBarPrefs.showHealthStatusBar ? '-o' : ''
                                                                            }`}
                                                                            style={{border: 'none'}}
                                                                            onClick={() => {
                                                                                healthBarPrefs.showHealthStatusBar = !healthBarPrefs.showHealthStatusBar;
                                                                                services.viewPreferences.updatePreferences({
                                                                                    appList: {
                                                                                        ...pref,
                                                                                        statusBarView: {
                                                                                            ...healthBarPrefs,
                                                                                            showHealthStatusBar: healthBarPrefs.showHealthStatusBar
                                                                                        }
                                                                                    }
                                                                                });
                                                                            }}>
                                                                            <i className={`fas fa-ruler-horizontal`} />
                                                                        </button>
                                                                    </Tooltip>
                                                                    <div className='applications-list__view-type' style={{marginLeft: 'auto'}}>
                                                                        <i
                                                                            className={classNames('fa fa-th', {selected: pref.view === Tiles}, 'menu_icon')}
                                                                            title='Tiles'
                                                                            onClick={() => {
                                                                                ctx.navigation.goto('.', {view: Tiles});
                                                                                services.viewPreferences.updatePreferences({appList: {...pref, view: Tiles}});
                                                                            }}
                                                                        />
                                                                        <i
                                                                            className={classNames('fa fa-th-list', {selected: pref.view === List}, 'menu_icon')}
                                                                            title='List'
                                                                            onClick={() => {
                                                                                ctx.navigation.goto('.', {view: List});
                                                                                services.viewPreferences.updatePreferences({appList: {...pref, view: List}});
                                                                            }}
                                                                        />
                                                                        <i
                                                                            className={classNames('fa fa-chart-pie', {selected: pref.view === Summary}, 'menu_icon')}
                                                                            title='Summary'
                                                                            onClick={() => {
                                                                                ctx.navigation.goto('.', {view: Summary});
                                                                                services.viewPreferences.updatePreferences({appList: {...pref, view: Summary}});
                                                                            }}
                                                                        />
                                                                    </div>
                                                                </React.Fragment>
                                                            ),
                                                            actionMenu: {
                                                                items:
                                                                    applications.length > 0 && isListOfApplications
                                                                        ? [
                                                                              {
                                                                                  title: 'New App',
                                                                                  iconClassName: 'fa fa-plus',
                                                                                  qeId: 'applications-list-button-new-app',
                                                                                  action: () => ctx.navigation.goto('.', {new: '{}'}, {replace: true})
                                                                              },
                                                                              {
                                                                                  title: 'Sync Apps',
                                                                                  iconClassName: 'fa fa-sync',
                                                                                  action: () => ctx.navigation.goto('.', {syncApps: true}, {replace: true})
                                                                              },
                                                                              {
                                                                                  title: 'Refresh Apps',
                                                                                  iconClassName: 'fa fa-redo',
                                                                                  action: () => ctx.navigation.goto('.', {refreshApps: true}, {replace: true})
                                                                              }
                                                                          ]
                                                                        : []
                                                            }
                                                        }}
                                                    />
                                                    <div className='applications-list'>
                                                        {applications.length === 0 && getProjectsFilter(pref)?.length === 0 && (pref.labelsFilter || []).length === 0 ? (
                                                            <EmptyState icon='argo-icon-application'>
                                                                <h4>No applications available to you just yet</h4>
                                                                <h5>Create new application to start managing resources in your cluster</h5>
                                                                <button
                                                                    qe-id='applications-list-button-create-application'
                                                                    className='argo-button argo-button--base'
                                                                    onClick={() => ctx.navigation.goto('.', {new: JSON.stringify({})}, {replace: true})}>
                                                                    Create application
                                                                </button>
                                                            </EmptyState>
                                                        ) : (
                                                            <>
                                                                {ReactDOM.createPortal(
                                                                    <DataLoader load={() => services.viewPreferences.getPreferences()}>
                                                                        {allpref => (
                                                                            <ApplicationsFilter
                                                                                apps={filterResults}
                                                                                onChange={newPrefs => onFilterPrefChanged(newPrefs)}
                                                                                pref={
                                                                                    isListOfApplications
                                                                                        ? (pref as AppsListPreferences & {page: number; search: string})
                                                                                        : (pref as AppSetsListPreferences)
                                                                                }
                                                                                collapsed={allpref.hideSidebar}
                                                                            />
                                                                        )}
                                                                    </DataLoader>,
                                                                    sidebarTarget?.current
                                                                )}

                                                                {(pref.view === 'summary' && <ApplicationsSummary applications={filteredApps} ctx={ctx} />) || (
                                                                    <Paginate
                                                                        header={filteredApps.length > 0 && <ApplicationsStatusBar applications={filteredApps} />}
                                                                        showHeader={healthBarPrefs.showHealthStatusBar}
                                                                        preferencesKey='applications-list'
                                                                        page={pref.page}
                                                                        emptyState={() => (
                                                                            <EmptyState icon='fa fa-search'>
                                                                                <h4>{getEmptyStateText}</h4>
                                                                                <h5>
                                                                                    Change filter criteria or&nbsp;
                                                                                    <a
                                                                                        onClick={() => {
                                                                                            if (isListOfApplications) {
                                                                                                AppsListPreferences.clearFilters(
                                                                                                    pref as AppsListPreferences & {page: number; search: string}
                                                                                                );
                                                                                            } else {
                                                                                                AppSetsListPreferences.clearFilters(pref as AppSetsListPreferences);
                                                                                            }
                                                                                            onFilterPrefChanged(pref);
                                                                                        }}>
                                                                                        clear filters
                                                                                    </a>
                                                                                </h5>
                                                                            </EmptyState>
                                                                        )}
                                                                        sortOptions={
                                                                            applications.length > 0 && isListOfApplications
                                                                                ? [
                                                                                      {title: 'Name', compare: (a, b) => a.metadata.name.localeCompare(b.metadata.name)},
                                                                                      {
                                                                                          title: 'Created At',
                                                                                          compare: (b, a) =>
                                                                                              a.metadata.creationTimestamp.localeCompare(b.metadata.creationTimestamp)
                                                                                      },
                                                                                      {
                                                                                          title: 'Synchronized',
                                                                                          compare: (b, a) =>
                                                                                              a.status.operationState?.finishedAt?.localeCompare(
                                                                                                  b.status.operationState?.finishedAt
                                                                                              )
                                                                                      }
                                                                                  ]
                                                                                : [
                                                                                      {title: 'Name', compare: (a, b) => a.metadata.name.localeCompare(b.metadata.name)},
                                                                                      {
                                                                                          title: 'Created At',
                                                                                          compare: (b, a) =>
                                                                                              a.metadata.creationTimestamp.localeCompare(b.metadata.creationTimestamp)
                                                                                      }
                                                                                  ]
                                                                        }
                                                                        data={filteredApps}
                                                                        onPageChange={page => ctx.navigation.goto('.', {page})}>
                                                                        {data =>
                                                                            (pref.view === 'tiles' && <ApplicationTiles {...abstractApplicationTilesProps(data)} />) || (
                                                                                <ApplicationsTable
                                                                                    applications={data}
                                                                                    syncApplication={(appName, appNamespace) =>
                                                                                        ctx.navigation.goto('.', {syncApp: appName, appNamespace}, {replace: true})
                                                                                    }
                                                                                    refreshApplication={refreshApp}
                                                                                    deleteApplication={(appName, appNamespace) =>
                                                                                        AppUtils.deleteApplication(appName, appNamespace, ctx)
                                                                                    }
                                                                                />
                                                                            )
                                                                        }
                                                                    </Paginate>
                                                                )}
                                                            </>
                                                        )}
                                                        {applications.length > 0 && isListOfApplications && (
                                                            <ApplicationsSyncPanel
                                                                key='syncsPanel'
                                                                show={syncAppsInput}
                                                                hide={() => ctx.navigation.goto('.', {syncApps: null}, {replace: true})}
                                                                apps={filteredApps}
                                                            />
                                                        )}
                                                        {applications.length > 0 && !isListOfApplications && (
                                                            <ApplicationsRefreshPanel
                                                                key='refreshPanel'
                                                                show={refreshAppsInput}
                                                                hide={() => ctx.navigation.goto('.', {refreshApps: null}, {replace: true})}
                                                                apps={filteredApps}
                                                            />
                                                        )}
                                                    </div>
                                                    {applications.length > 0 && isListOfApplications && (
                                                        <ObservableQuery>
                                                            {q => (
                                                                <DataLoader
                                                                    load={() =>
                                                                        q.pipe(
                                                                            mergeMap(params => {
                                                                                const syncApp = params.get('syncApp');
                                                                                const appNamespace = params.get('appNamespace');
                                                                                return (
                                                                                    (syncApp &&
                                                                                        from(services.applications.get(syncApp, appNamespace, ctx.history.location.pathname))) ||
                                                                                    from([null])
                                                                                );
                                                                            })
                                                                        )
                                                                    }>
                                                                    {app => (
                                                                        <ApplicationSyncPanel
                                                                            key='syncPanel'
                                                                            application={app}
                                                                            selectedResource={'all'}
                                                                            hide={() => ctx.navigation.goto('.', {syncApp: null}, {replace: true})}
                                                                        />
                                                                    )}
                                                                </DataLoader>
                                                            )}
                                                        </ObservableQuery>
                                                    )}
                                                    <SlidingPanel
                                                        isShown={!!appInput}
                                                        onClose={() => ctx.navigation.goto('.', {new: null}, {replace: true})}
                                                        header={
                                                            <div>
                                                                {applications.length > 0 && isListOfApplications && (
                                                                    <button
                                                                        qe-id='applications-list-button-create'
                                                                        className='argo-button argo-button--base'
                                                                        disabled={isAppCreatePending}
                                                                        onClick={() => createApi && createApi.submitForm(null)}>
                                                                        <Spinner show={isAppCreatePending} style={{marginRight: '5px'}} />
                                                                        Create
                                                                    </button>
                                                                )}{' '}
                                                                <button
                                                                    qe-id='applications-list-button-cancel'
                                                                    onClick={() => ctx.navigation.goto('.', {new: null}, {replace: true})}
                                                                    className='argo-button argo-button--base-o'>
                                                                    Cancel
                                                                </button>
                                                            </div>
                                                        }>
                                                        {appInput && isListOfApplications && (
                                                            <ApplicationCreatePanel
                                                                getFormApi={api => {
                                                                    setCreateApi(api);
                                                                }}
                                                                createApp={async app => {
                                                                    setAppCreatePending(true);
                                                                    try {
                                                                        await services.applications.create(app);
                                                                        ctx.navigation.goto('.', {new: null}, {replace: true});
                                                                    } catch (e) {
                                                                        ctx.notifications.show({
                                                                            content: <ErrorNotification title='Unable to create application' e={e} />,
                                                                            type: NotificationType.Error
                                                                        });
                                                                    } finally {
                                                                        setAppCreatePending(false);
                                                                    }
                                                                }}
                                                                app={appInput}
                                                                onAppChanged={app => ctx.navigation.goto('.', {new: JSON.stringify(app)}, {replace: true})}
                                                            />
                                                        )}
                                                    </SlidingPanel>
                                                </React.Fragment>
                                            );
                                        }}
                                    </DataLoader>
                                </Page>
                            )}
                        </ViewPref>
                    )}
                </Consumer>
            </KeybindingProvider>
        </ClusterCtx.Provider>
    );
};
