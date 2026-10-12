export type ProfileAction =
  | { readonly kind: 'confirm'; readonly username: string; readonly display_name: string }
  | { readonly kind: 'display'; readonly display_name: string }
  | { readonly kind: 'username'; readonly username: string };
