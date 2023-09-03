import { Tooltip } from 'argo-ui/v2';
import * as React from 'react';
import { COLORS } from '../../../shared/components';
import { Consumer } from '../../../shared/context';
import * as models from '../../../shared/models';

import './applications-status-bar.scss';
import { isAppSet } from '../utils';

export interface ApplicationsStatusBarProps {
    applications: models.AbstractApplication[];
}

export interface ApplicationsReadings {
    name: string,
    value: number,
    color: string
}

export function getAbstractReadings(applications: models.AbstractApplication[]): ApplicationsReadings[] {
    const readings =
        [
            {
                name: 'Healthy',
                value: applications.filter(app => app.status.health.status === 'Healthy').length,
                color: COLORS.health.healthy
            },
            {
                name: 'Degraded',
                value: applications.filter(app => app.status.health.status === 'Degraded').length,
                color: COLORS.health.degraded
            },
            {
                name: 'Unknown',
                value: applications.filter(app => app.status.health.status === 'Unknown').length,
                color: COLORS.health.unknown
            }];
    return readings
}

export function getApplicationReadings(applications: models.AbstractApplication[]): ApplicationsReadings[] {
    const readings = [
        {
            name: 'Progressing',
            value: applications.filter(app => app.status.health.status === 'Progressing').length,
            color: COLORS.health.progressing
        },

        {
            name: 'Suspended',
            value: applications.filter(app => app.status.health.status === 'Suspended').length,
            color: COLORS.health.suspended
        },
        {
            name: 'Missing',
            value: applications.filter(app => app.status.health.status === 'Missing').length,
            color: COLORS.health.missing
        },

    ];
    return readings
}

export const ApplicationsStatusBar = ({ applications }: ApplicationsStatusBarProps) => {
    const IS_APPSET = isAppSet(applications[0])
    const readings = IS_APPSET ? getAbstractReadings(applications) : getAbstractReadings(applications).concat(getApplicationReadings(applications))

    // will sort readings by value greatest to lowest, then by name
    readings.sort((a, b) => (a.value < b.value ? 1 : a.value === b.value ? (a.name > b.name ? 1 : -1) : -1));

    const totalItems = readings.reduce((total, i) => {
        return total + i.value;
    }, 0);

    return (
        <Consumer>
            {ctx => (
                <>
                    {totalItems > 1 && (
                        <div className='status-bar'>
                            {readings &&
                                readings.length > 1 &&
                                readings.map((item, i) => {
                                    if (item.value > 0) {
                                        return (
                                            <div className='status-bar__segment' style={{ backgroundColor: item.color, width: (item.value / totalItems) * 100 + '%' }} key={i}>
                                                <Tooltip content={`${item.value} ${item.name}`} inverted={true}>
                                                    <div className='status-bar__segment__fill' />
                                                </Tooltip>
                                            </div>
                                        );
                                    }
                                })}
                        </div>
                    )}
                </>
            )}
        </Consumer>
    );
};
