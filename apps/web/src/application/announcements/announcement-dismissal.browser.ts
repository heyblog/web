type DismissalStorage = Pick<Storage, 'getItem' | 'setItem'>;
type GetStorage = () => DismissalStorage;

const browserStorage: GetStorage = () => window.localStorage;
const keyPrefix = 'heyblog:announcement:dismissed:';

export function isAnnouncementDismissed(
  id: string,
  getStorage: GetStorage = browserStorage,
): boolean {
  try {
    return getStorage().getItem(`${keyPrefix}${id}`) === '1';
  } catch {
    return false;
  }
}

export function dismissAnnouncement(id: string, getStorage: GetStorage = browserStorage): void {
  try {
    getStorage().setItem(`${keyPrefix}${id}`, '1');
  } catch {
    // Closing the current banner still works when persistence is unavailable.
  }
}
