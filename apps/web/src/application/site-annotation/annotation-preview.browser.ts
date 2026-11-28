const PREVIEW_WINDOW_NAME = 'heyblog-annotation-preview';
const PREVIEW_LINKED_KEY = 'heyblog:annotation:preview-linked';

let previewWindow: Window | null = null;

function screenGeometry(): { left: number; top: number; width: number; height: number } {
  const screenObj = window.screen as Screen & { availLeft?: number; availTop?: number };
  const availLeft = typeof screenObj.availLeft === 'number' ? screenObj.availLeft : 0;
  const availTop = typeof screenObj.availTop === 'number' ? screenObj.availTop : 0;
  const availWidth = Math.max(window.screen.availWidth || window.outerWidth || 1280, 800);
  const availHeight = Math.max(window.screen.availHeight || window.outerHeight || 800, 600);
  const width = Math.max(Math.round(availWidth * 0.52), 560);
  const left = availLeft + Math.max(availWidth - width, 0);
  return { left, top: availTop, width, height: availHeight };
}

export function readPreviewLinkedPreference(): boolean {
  try {
    return window.sessionStorage.getItem(PREVIEW_LINKED_KEY) === '1';
  } catch {
    return false;
  }
}

export function writePreviewLinkedPreference(linked: boolean): void {
  try {
    if (linked) window.sessionStorage.setItem(PREVIEW_LINKED_KEY, '1');
    else window.sessionStorage.removeItem(PREVIEW_LINKED_KEY);
  } catch {
    // 忽略无痕模式或存储不可用
  }
}

export function isPreviewWindowOpen(): boolean {
  return Boolean(previewWindow && !previewWindow.closed);
}

/**
 * 在屏幕右侧打开或复用命名预览窗口，绕开 iframe 嵌入限制。
 */
export function openPreviewWindow(url: string, options?: { focus?: boolean }): boolean {
  const target = url.trim();
  if (!/^https?:\/\//i.test(target)) return false;

  if (previewWindow && !previewWindow.closed) {
    try {
      previewWindow.location.href = target;
      if (options?.focus) previewWindow.focus();
      return true;
    } catch {
      // 跨域窗口引用降级为同名 window.open 导航
    }
  }

  const { left, top, width, height } = screenGeometry();
  const features = `popup=yes,width=${width},height=${height},left=${left},top=${top},resizable=yes,scrollbars=yes`;
  const opened = window.open(target, PREVIEW_WINDOW_NAME, features);
  if (!opened) return false;
  previewWindow = opened;
  if (options?.focus) opened.focus();
  return true;
}

export function closePreviewWindow(): void {
  if (previewWindow && !previewWindow.closed) {
    previewWindow.close();
  }
  previewWindow = null;
  writePreviewLinkedPreference(false);
}
