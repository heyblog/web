/// <reference types="astro/client" />

declare const __HEYBLOG_BUILD_METADATA__: import('./shared/integrations/build-metadata').BuildMetadata;

declare namespace App {
  interface Locals {
    sessionUser?: import('./api/auth/auth.types').SessionUser | null;
  }
}
