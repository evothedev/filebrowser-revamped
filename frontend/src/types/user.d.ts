interface IUser {
  id: number;
  username: string;
  password: string;
  scope: string;
  locale: string;
  perm: Permissions;
  commands: string[];
  rules: IRule[];
  lockPassword: boolean;
  hideDotfiles: boolean;
  singleClick: boolean;
  redirectAfterCopyMove: boolean;
  dateFormat: boolean;
  viewMode: ViewModeType;
  sorting?: Sorting;
  aceEditorTheme: string;
}

type ViewModeType = "list" | "mosaic" | "mosaic gallery";

interface IUserForm {
  id?: number;
  username?: string;
  password?: string;
  scope?: string;
  locale?: string;
  perm?: Permissions;
  commands?: string[];
  rules?: IRule[];
  lockPassword?: boolean;
  hideDotfiles?: boolean;
  singleClick?: boolean;
  redirectAfterCopyMove?: boolean;
  dateFormat?: boolean;
}

interface Permissions {
  admin: boolean;
  copy: boolean;
  create: boolean;
  delete: boolean;
  download: boolean;
  execute: boolean;
  modify: boolean;
  move: boolean;
  rename: boolean;
  share: boolean;
  shell: boolean;
  upload: boolean;
}

interface Sorting {
  by: string;
  asc: boolean;
}

interface IRule {
  allow: boolean;
  path: string;
  regex: boolean;
  regexp: IRegexp;
}

interface IRegexp {
  raw: string;
}

// "" follows the operating system's colour scheme. The rest are the themes in
// utils/themes.ts, each of which is both a CSS class and the value stored in
// settings.branding.theme.
type UserTheme =
  | ""
  | "light"
  | "dark"
  | "nord-light"
  | "nord-dark"
  | "solarized-light"
  | "solarized-dark"
  | "gruvbox-light"
  | "gruvbox-dark"
  | "dracula";
