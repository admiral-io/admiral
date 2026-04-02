export interface MenuState {
  selectedItem: string[];
  selectedID: string | null;
  drawerOpen: boolean;
  menu: Record<string, unknown>;
}
