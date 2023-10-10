import {MockupList} from 'argo-ui';
import * as React from 'react';

import {DataLoader, EventsList} from '../../../shared/components';
import {services} from '../../../shared/services';

export const ApplicationResourceEvents = (props: {isApp: boolean, applicationName: string; applicationNamespace: string; resource?: {namespace: string; name: string; uid: string}}) => (
    <div className='application-resource-events'>
        <DataLoader
            load={() =>
                props.resource
                    ? services.applications.resourceEvents(props.isApp, props.applicationName, props.applicationNamespace, props.resource)
                    : services.applications.events(props.isApp, props.applicationName, props.applicationNamespace)
            }
            loadingRenderer={() => <MockupList height={50} marginTop={10} />}>
            {events => <EventsList events={events} />}
        </DataLoader>
    </div>
);
