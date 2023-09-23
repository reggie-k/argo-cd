import { DataLoader, Tooltip } from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import { Key, KeybindingContext, NumKey, NumKeyToNumber, NumPadKey, useNav } from 'argo-ui/v2';
import { Cluster } from '../../../shared/components';
import { Consumer, Context, AuthSettingsCtx } from '../../../shared/context';
import * as models from '../../../shared/models';
import { ApplicationURLs } from '../application-urls';
import * as AppUtils from '../utils';
import { getAppDefaultSource, OperationState } from '../utils';
import { services } from '../../../shared/services';

import './applications-tiles.scss';

export interface ApplicationTilesProps extends AbstractApplicationTilesProps {
    applications: models.Application[];
    syncApplication: (appName: string, appNamespace: string) => any;
    refreshApplication: (appName: string, appNamespace: string) => any;
}

export interface AbstractApplicationTilesProps {
    applications: models.AbstractApplication[];
    deleteApplication: (appName: string, appNamespace: string) => any;
}

export interface ApplicationSetTilesProps extends AbstractApplicationTilesProps {
    applications: models.ApplicationSet[];
}

const useItemsPerContainer = (itemRef: any, containerRef: any): number => {
    const [itemsPer, setItemsPer] = React.useState(0);

    React.useEffect(() => {
        const handleResize = () => {
            let timeoutId: any;
            clearTimeout(timeoutId);
            timeoutId = setTimeout(() => {
                timeoutId = null;
                const itemWidth = itemRef.current ? itemRef.current.offsetWidth : -1;
                const containerWidth = containerRef.current ? containerRef.current.offsetWidth : -1;
                const curItemsPer = containerWidth > 0 && itemWidth > 0 ? Math.floor(containerWidth / itemWidth) : 1;
                if (curItemsPer !== itemsPer) {
                    setItemsPer(curItemsPer);
                }
            }, 1000);
        };
        window.addEventListener('resize', handleResize);
        handleResize();
        return () => {
            window.removeEventListener('resize', handleResize);
        };
    }, []);

    return itemsPer || 1;
};

export const ApplicationTiles = (tilesProps: AbstractApplicationTilesProps) => {
    const [selectedApp, navApp, reset] = useNav(tilesProps.applications.length);

    const ctxh = React.useContext(Context);
    const appRef = { ref: React.useRef(null), set: false };
    const appContainerRef = React.useRef(null);
    const appsPerRow = useItemsPerContainer(appRef.ref, appContainerRef);
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);

    const { useKeybinding } = React.useContext(KeybindingContext);

    useKeybinding({ keys: Key.RIGHT, action: () => navApp(1) });
    useKeybinding({ keys: Key.LEFT, action: () => navApp(-1) });
    useKeybinding({ keys: Key.DOWN, action: () => navApp(appsPerRow) });
    useKeybinding({ keys: Key.UP, action: () => navApp(-1 * appsPerRow) });

    useKeybinding({
        keys: Key.ENTER,
        action: () => {
            if (selectedApp > -1) {
                ctxh.navigation.goto(`${AppUtils.getRootPath(tilesProps.applications[selectedApp])}${tilesProps.applications[selectedApp].metadata.name}`);
                return true;
            }
            return false;
        }
    });

    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            if (selectedApp > -1) {
                reset();
                return true;
            }
            return false;
        }
    });

    useKeybinding({
        keys: Object.values(NumKey) as NumKey[],
        action: n => {
            reset();
            return navApp(NumKeyToNumber(n));
        }
    });
    useKeybinding({
        keys: Object.values(NumPadKey) as NumPadKey[],
        action: n => {
            reset();
            return navApp(NumKeyToNumber(n));
        }
    });
    return (
        <Consumer>
            {ctx => (
                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                    {pref => {
                        const favList = pref.appList.favoritesAppList || [];
                        return (
                            <div
                                className='applications-tiles argo-table-list argo-table-list--clickable row small-up-1 medium-up-2 large-up-3 xxxlarge-up-4'
                                ref={appContainerRef}>
                                {tilesProps.applications.map((app, i) => {
                                    const source = getAppDefaultSource(app);
                                    return (
                                        <div key={AppUtils.appInstanceName(app)} className='column column-block'>
                                            {/* {!AppUtils.isAppSet(app) && <AppUtils.HealthStatusIcon state={(app as models.Application).status.health} />} {!AppUtils.isAppSet(app) && (app as models.Application).status.health.status}
                                                                {AppUtils.isAppSet(app) && <AppUtils.AppSetHealthStatusIcon state={(app as models.ApplicationSet).status} />} {AppUtils.isAppSet(app) && (app as models.ApplicationSet).status.conditions[0].status}
                                                                 */}
                                            <div
                                                ref={appRef.set ? null : appRef.ref}
                                                // sortOptions={AppUtils.isAppSet(applications[0]) ? [
                                                className={!AppUtils.isAppSet(app) ? `argo-table-list__row applications-list__entry applications-list__entry--health-${(app as models.Application).status.health.status} ${selectedApp === i ? 'applications-tiles__selected' : ''
                                                    }` : `argo-table-list__row applications-list__entry applications-list__entry--health-${(app as models.ApplicationSet).status.conditions[0].status} ${selectedApp === i ? 'applications-tiles__selected' : ''
                                                }`
                                             } >
                                                <div
                                                    className='row'
                                                    onClick={e =>
                                                        ctx.navigation.goto(
                                                            `${AppUtils.getRootPath(app)}${app.metadata.namespace}/${app.metadata.name}`,
                                                            { view: pref.appDetails.view },
                                                            { event: e }
                                                        )
                                                    }>
                                                    <div className={`columns small-12 applications-list__info qe-applications-list-${AppUtils.appInstanceName(app)}`}>
                                                        <div className='row'>
                                                            {!AppUtils.isAppSet(app) && <div className={app.status.summary.externalURLs?.length > 0 ? 'columns small-10' : 'columns small-11'}>
                                                                <i className={'icon argo-icon-' + (source.chart != null ? 'helm' : 'git')} />
                                                                <Tooltip content={AppUtils.appInstanceName(app)}>
                                                                    <span className='applications-list__title'>
                                                                        {AppUtils.appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}
                                                                    </span>
                                                                </Tooltip>
                                                            </div>}
                                                            {!AppUtils.isAppSet(app) && <div className={app.status.summary.externalURLs?.length > 0 ? 'columns small-2' : 'columns small-1'}>
                                                                <div className='applications-list__external-link'>
                                                                    <ApplicationURLs urls={app.status.summary.externalURLs} />
                                                                    <Tooltip content={favList?.includes(app.metadata.name) ? 'Remove Favorite' : 'Add Favorite'}>
                                                                        <button
                                                                            className='large-text-height'
                                                                            onClick={e => {
                                                                                e.stopPropagation();
                                                                                favList?.includes(app.metadata.name)
                                                                                    ? favList.splice(favList.indexOf(app.metadata.name), 1)
                                                                                    : favList.push(app.metadata.name);
                                                                                services.viewPreferences.updatePreferences({ appList: { ...pref.appList, favoritesAppList: favList } });
                                                                            }}>
                                                                            <i
                                                                                className={favList?.includes(app.metadata.name) ? 'fas fa-star fa-lg' : 'far fa-star fa-lg'}
                                                                                style={{
                                                                                    cursor: 'pointer',
                                                                                    marginLeft: '7px',
                                                                                    color: favList?.includes(app.metadata.name) ? '#FFCE25' : '#8fa4b1'
                                                                                }}
                                                                            />
                                                                        </button>
                                                                    </Tooltip>
                                                                </div>
                                                            </div>}
                                                        </div>
                                                        {!AppUtils.isAppSet(app) && <div className='row'>
                                                            <div className='columns small-3' title='Project:'>
                                                                Project:
                                                            </div>
                                                            <div className='columns small-9'>{app.spec.project}</div>
                                                        </div>}
                                                        <div className='row'>
                                                            <div className='columns small-3' title='Labels:'>
                                                                Labels:
                                                            </div>
                                                            <div className='columns small-9'>
                                                                <Tooltip
                                                                    zIndex={4}
                                                                    content={
                                                                        <div>
                                                                            {Object.keys(app.metadata.labels || {})
                                                                                .map(label => ({ label, value: app.metadata.labels[label] }))
                                                                                .map(item => (
                                                                                    <div key={item.label}>
                                                                                        {item.label}={item.value}
                                                                                    </div>
                                                                                ))}
                                                                        </div>
                                                                    }>
                                                                    <span>
                                                                        {Object.keys(app.metadata.labels || {})
                                                                            .map(label => `${label}=${app.metadata.labels[label]}`)
                                                                            .join(', ')}
                                                                    </span>
                                                                </Tooltip>
                                                            </div>
                                                        </div>
                                                        <div className='row'>
                                                            <div className='columns small-3' title='Status:'>
                                                                Status:
                                                            </div>
                                                            <div className='columns small-9' qe-id='applications-tiles-health-status'>
                                                                {!AppUtils.isAppSet(app) && <AppUtils.HealthStatusIcon state={(app as models.Application).status.health} />} {!AppUtils.isAppSet(app) && (app as models.Application).status.health.status}
                                                                {AppUtils.isAppSet(app) && <AppUtils.AppSetHealthStatusIcon state={(app as models.ApplicationSet).status} />} {AppUtils.isAppSet(app) && (app as models.ApplicationSet).status.conditions[0].status}
                                                                &nbsp;
                                                                {!AppUtils.isAppSet(app) && <AppUtils.ComparisonStatusIcon status={(app as models.Application).status.sync.status} />} {!AppUtils.isAppSet(app) && (app as models.Application).status.sync.status}
                                                                &nbsp;
                                                                {!AppUtils.isAppSet(app) && <OperationState app={app} quiet={true} />}
                                                            </div>
                                                        </div>
                                                        {!AppUtils.isAppSet(app) &&
                                                            (<div className='row'>
                                                                <div className='columns small-3' title='Repository:'>
                                                                    Repository:
                                                                </div>
                                                                <div className='columns small-9'>
                                                                    <Tooltip content={source.repoURL} zIndex={4}>
                                                                        <span>{source.repoURL}</span>
                                                                    </Tooltip>
                                                                </div>
                                                            </div>
                                                            )}
                                                        {!AppUtils.isAppSet(app) &&
                                                            (<div className='row'>
                                                                <div className='columns small-3' title='Target Revision:'>
                                                                    Target Revision:
                                                                </div>
                                                                <div className='columns small-9'>{source.targetRevision || 'HEAD'}</div>
                                                            </div>
                                                            )}
                                                        {!AppUtils.isAppSet(app) && source.path &&  (
                                                            <div className='row'>
                                                                <div className='columns small-3' title='Path:'>
                                                                    Path:
                                                                </div>
                                                                <div className='columns small-9'>{source.path}</div>
                                                            </div>
                                                        )}
                                                        {!AppUtils.isAppSet(app) && source.chart && (
                                                            <div className='row'>
                                                                <div className='columns small-3' title='Chart:'>
                                                                    Chart:
                                                                </div>
                                                                <div className='columns small-9'>{source.chart}</div>
                                                            </div>
                                                        )}
                                                        {!AppUtils.isAppSet(app) && (
                                                            <div className='row'>
                                                                <div className='columns small-3' title='Destination:'>
                                                                    Destination:
                                                                </div>
                                                                <div className='columns small-9'>
                                                                    <Cluster server={(app as models.Application).spec.destination.server} name={app.spec.destination.name} />
                                                                </div>
                                                            </div>
                                                        )}
                                                        {!AppUtils.isAppSet(app) && (
                                                            <div className='row'>
                                                                <div className='columns small-3' title='Namespace:'>
                                                                    Namespace:
                                                                </div>
                                                                <div className='columns small-9'>{(app as models.Application).spec.destination.namespace}</div>
                                                            </div>)}

                                                        <div className='row'>
                                                            <div className='columns small-3' title='Age:'>
                                                                Created At:
                                                            </div>
                                                            <div className='columns small-9'>{AppUtils.formatCreationTimestamp(app.metadata.creationTimestamp)}</div>
                                                        </div>
                                                        {!AppUtils.isAppSet(app) && (app as models.Application).status.operationState && (
                                                            <div className='row'>
                                                                <div className='columns small-3' title='Last sync:'>
                                                                    Last Sync:
                                                                </div>
                                                                <div className='columns small-9'>
                                                                    {AppUtils.formatCreationTimestamp((app as models.Application).status.operationState.finishedAt || app.status.operationState.startedAt)}
                                                                </div>
                                                            </div>
                                                        )}

                                                        <div className='row'>
                                                            <div className='columns applications-list__entry--actions'>
                                                                {!AppUtils.isAppSet(app) && (
                                                                    <a
                                                                        className='argo-button argo-button--base'
                                                                        qe-id='applications-tiles-button-sync'
                                                                        onClick={e => {
                                                                            e.stopPropagation();
                                                                            (tilesProps as ApplicationTilesProps).syncApplication(app.metadata.name, app.metadata.namespace);
                                                                        }}>
                                                                        <i className='fa fa-sync' /> Sync
                                                                    </a>
                                                                )}
                                                                &nbsp;
                                                                {!AppUtils.isAppSet(app) &&
                                                                    (<a
                                                                        className='argo-button argo-button--base'
                                                                        qe-id='applications-tiles-button-refresh'
                                                                        {...AppUtils.refreshLinkAttrs(app)}
                                                                        onClick={e => {
                                                                            e.stopPropagation();
                                                                            (tilesProps as ApplicationTilesProps).refreshApplication(app.metadata.name, app.metadata.namespace);
                                                                        }}>
                                                                        <i className={classNames('fa fa-redo', { 'status-icon--spin': AppUtils.isAppRefreshing(app) })} />{' '}
                                                                        <span className='show-for-xxlarge'>Refresh</span>
                                                                    </a>
                                                                    )}
                                                                &nbsp;
                                                                <a
                                                                    className='argo-button argo-button--base'
                                                                    qe-id='applications-tiles-button-delete'
                                                                    onClick={e => {
                                                                        e.stopPropagation();
                                                                        (tilesProps as ApplicationTilesProps).deleteApplication(app.metadata.name, app.metadata.namespace);
                                                                    }}>
                                                                    <i className='fa fa-times-circle' /> <span className='show-for-xxlarge'>Delete</span>
                                                                </a>
                                                            </div>
                                                        </div>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    );
                                })}
                            </div>
                        );
                    }}
                </DataLoader>
            )}
        </Consumer>
    );
};
