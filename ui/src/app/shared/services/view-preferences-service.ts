import * as deepMerge from 'deepmerge';
import {BehaviorSubject, Observable} from 'rxjs';

import {PodGroupType} from '../../applications/components/application-pod-view/pod-view';
import {UserMessages} from '../models';

export type AppsDetailsViewType = 'tree' | 'network' | 'list' | 'pods';

export enum AppsDetailsViewKey {
    Tree = 'tree',
    Network = 'network',
    List = 'list',
    Pods = 'pods'
}

export type AppSetsDetailsViewType = 'tree' | 'list';

export enum AppSetsDetailsViewKey {
    Tree = 'tree',
    List = 'list'
}

export interface AbstractAppDetailsPreferences {
    resourceFilter: string[];
    darkMode: boolean;
    hideFilters: boolean;
    groupNodes?: boolean;
    zoom: number;
    view: AppsDetailsViewType | AppSetsDetailsViewType | string;
    resourceView: 'manifest' | 'diff' | 'desiredManifest';
    inlineDiff: boolean;
    compactDiff: boolean;
    hideManagedFields?: boolean;
    orphanedResources: boolean;
}

export interface AppDetailsPreferences extends AbstractAppDetailsPreferences {
    view: AppsDetailsViewType | string;
    podView: PodViewPreferences;
    followLogs: boolean;
    wrapLines: boolean;
    podGroupCount: number;
    userHelpTipMsgs: UserMessages[];
}

export interface AppSetDetailsPreferences extends AbstractAppDetailsPreferences {
    view: AppSetsDetailsViewType | string;
}

export interface PodViewPreferences {
    sortMode: PodGroupType;
    hideUnschedulable: boolean;
}

export interface HealthStatusBarPreferences {
    showHealthStatusBar: boolean;
}

export type AppsListViewType = 'tiles' | 'list' | 'summary';

export enum AppsListViewKey {
    List = 'list',
    Summary = 'summary',
    Tiles = 'tiles'
}

export class AbstractAppsListPreferences {
    public static clearFilters(pref: AbstractAppsListPreferences) {
        pref.healthFilter = [];
        pref.labelsFilter = [];
        pref.showFavorites = false;
    }

    public labelsFilter: string[];
    public healthFilter: string[];
    public view: AppsListViewType;
    public hideFilters: boolean;
    public statusBarView: HealthStatusBarPreferences;
    public showFavorites: boolean;
    public favoritesAppList: string[];
}

export class AppsListPreferences extends AbstractAppsListPreferences {
    public static countEnabledFilters(pref: AppsListPreferences) {
        return [pref.clustersFilter, pref.healthFilter, pref.labelsFilter, pref.namespacesFilter, pref.projectsFilter, pref.reposFilter, pref.syncFilter].reduce(
            (count, filter) => {
                if (filter && filter.length > 0) {
                    return count + 1;
                }
                return count;
            },
            0
        );
    }

    public static clearFilters(pref: AppsListPreferences) {
        super.clearFilters(pref);

        pref.clustersFilter = [];
        pref.namespacesFilter = [];
        pref.projectsFilter = [];
        pref.reposFilter = [];
        pref.syncFilter = [];
        pref.autoSyncFilter = [];
    }

    public projectsFilter: string[];
    public reposFilter: string[];
    public syncFilter: string[];
    public autoSyncFilter: string[];
    public namespacesFilter: string[];
    public clustersFilter: string[];
}

export class AppSetsListPreferences extends AbstractAppsListPreferences {
    public static countEnabledFilters(pref: AppSetsListPreferences) {
        return [pref.healthFilter, pref.labelsFilter].reduce((count, filter) => {
            if (filter && filter.length > 0) {
                return count + 1;
            }
            return count;
        }, 0);
    }

    public static clearFilters(pref: AppSetsListPreferences) {
        super.clearFilters(pref);
    }
}

export interface AbstractViewPreferences {
    version: number;
    pageSizes: {[key: string]: number};
    sortOptions?: {[key: string]: string};
    hideBannerContent: string;
    hideSidebar: boolean;
    position: string;
    theme: string;
    appDetails: AbstractAppDetailsPreferences;
    appList: AbstractAppsListPreferences;
}

export interface ViewPreferences extends AbstractViewPreferences {
    appDetails: AppDetailsPreferences;
    appList: AppsListPreferences;
}

export interface AppSetViewPreferences extends AbstractViewPreferences {
    appDetails: AppSetDetailsPreferences;
    appList: AppSetsListPreferences;
}

const VIEW_PREFERENCES_KEY = 'view_preferences';

const minVer = 5;

// Default view preferences for apps and for appsets should probably differ, and while constructing them,
// it should allegedly be clear what they are used for (apps or appsets)
// In reality, I didn't see any behavior difference for initializing the default preferences with any of them.
// So this is here to indicate that if a use case, in which the behaviour WOULD be different, would be found,
// isForApps would need to be calculated and then the separation is already in place.
const isForApps = true;
const DEFAULT_PREFERENCES: ViewPreferences | AppSetViewPreferences = isForApps
    ? {
          version: 1,
          appDetails: {
              view: 'tree',
              hideFilters: false,
              resourceFilter: [],
              inlineDiff: false,
              compactDiff: false,
              hideManagedFields: true,
              resourceView: 'manifest',
              orphanedResources: false,
              podView: {
                  sortMode: 'node',
                  hideUnschedulable: true
              },
              darkMode: false,
              followLogs: false,
              wrapLines: false,
              zoom: 1.0,
              podGroupCount: 15.0,
              userHelpTipMsgs: []
          },
          appList: {
              view: 'tiles' as AppsListViewType,
              labelsFilter: new Array<string>(),
              projectsFilter: new Array<string>(),
              namespacesFilter: new Array<string>(),
              clustersFilter: new Array<string>(),
              reposFilter: new Array<string>(),
              syncFilter: new Array<string>(),
              autoSyncFilter: new Array<string>(),
              healthFilter: new Array<string>(),
              hideFilters: false,
              showFavorites: false,
              favoritesAppList: new Array<string>(),
              statusBarView: {
                  showHealthStatusBar: true
              }
          },
          pageSizes: {},
          hideBannerContent: '',
          hideSidebar: false,
          position: '',
          theme: 'light'
      }
    : {
          version: 1,
          appDetails: {
              view: 'tree',
              hideFilters: false,
              resourceFilter: [],
              inlineDiff: false,
              compactDiff: false,
              hideManagedFields: true,
              resourceView: 'manifest',
              orphanedResources: false,
              darkMode: false,
              zoom: 1.0
          },
          appList: {
              view: 'tiles' as AppsListViewType,
              labelsFilter: new Array<string>(),
              healthFilter: new Array<string>(),
              hideFilters: false,
              showFavorites: false,
              favoritesAppList: new Array<string>(),
              statusBarView: {
                  showHealthStatusBar: true
              }
          },
          pageSizes: {},
          hideBannerContent: '',
          hideSidebar: false,
          position: '',
          theme: 'light'
      };

export function isAppSetViewPreferences(pref: AbstractViewPreferences) {
    // There must be a more elegant way of determining that
    return !('followLogs' in pref.appDetails);
}

export class ViewPreferencesService {
    private preferencesSubj: BehaviorSubject<AbstractViewPreferences>;

    public init() {
        if (!this.preferencesSubj) {
            this.preferencesSubj = new BehaviorSubject(this.loadPreferences());
            window.addEventListener('storage', () => {
                this.preferencesSubj.next(this.loadPreferences());
            });
        }
    }

    public getPreferences(): Observable<AbstractViewPreferences> {
        return this.preferencesSubj;
    }

    public updatePreferences(change: Partial<ViewPreferences> | Partial<AppSetViewPreferences>) {
        const nextPref = Object.assign({}, this.preferencesSubj.getValue(), change, {version: minVer});
        window.localStorage.setItem(VIEW_PREFERENCES_KEY, JSON.stringify(nextPref));
        this.preferencesSubj.next(nextPref);
    }

    private loadPreferences(): AbstractViewPreferences {
        let preferences: AbstractViewPreferences;
        const preferencesStr = window.localStorage.getItem(VIEW_PREFERENCES_KEY);
        if (preferencesStr) {
            try {
                preferences = JSON.parse(preferencesStr);
            } catch (e) {
                preferences = DEFAULT_PREFERENCES;
            }
            if (!preferences.version || preferences.version < minVer) {
                preferences = DEFAULT_PREFERENCES;
            }
        } else {
            preferences = DEFAULT_PREFERENCES;
        }
        return deepMerge(DEFAULT_PREFERENCES, preferences);
    }
}
