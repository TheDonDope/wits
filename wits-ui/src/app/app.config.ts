import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { ApplicationConfig, isDevMode, provideBrowserGlobalErrorListeners } from '@angular/core';
import { MAT_ICON_DEFAULT_OPTIONS } from '@angular/material/icon';
import { provideRouter, withComponentInputBinding, withViewTransitions } from '@angular/router';
import { provideServiceWorker } from '@angular/service-worker';

import { routes } from './app.routes';
import { engineInterceptor } from './data/engine';
import { ENGINE } from './data/mode';

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    provideRouter(routes, withComponentInputBinding(), withViewTransitions()),
    // On the phone, the API is answered in the page (data/engine.ts).
    provideHttpClient(withInterceptors([engineInterceptor])),
    // The icons ship with the app, as the rounded Material Symbols.
    { provide: MAT_ICON_DEFAULT_OPTIONS, useValue: { fontSet: 'material-symbols-rounded' } },
    // The phone's build works offline: everything it needs, the engine
    // included, is cached on the first visit.
    provideServiceWorker('ngsw-worker.js', { enabled: ENGINE && !isDevMode(), registrationStrategy: 'registerImmediately' }),
  ],
};
