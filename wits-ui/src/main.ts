import { bootstrapApplication } from '@angular/platform-browser';
import { appConfig } from './app/app.config';
import { App } from './app/app';
import { apply } from './app/data/theme';

// The stored theme goes on before Angular starts, so the page never flashes
// the default first.
apply();

bootstrapApplication(App, appConfig).catch((err: unknown) => {
  console.error(err);
});
