import type { FunctionComponent, ReactNode } from 'react';
import type { SvgIconTypeMap, ChipProps } from '@mui/material';
import type { OverridableComponent } from '@mui/material/OverridableComponent';

export type OverrideIcon =
  | (OverridableComponent<SvgIconTypeMap<Record<string, unknown>, 'svg'>> & {
      muiName: string;
    })
  | React.ComponentClass<Record<string, unknown>>
  | FunctionComponent<Record<string, unknown>>
  | React.ComponentType;

export interface GenericCardProps {
  title?: string;
  primary?: string | number | undefined;
  secondary?: string;
  content?: string;
  image?: string;
  dateTime?: string;
  iconPrimary?: OverrideIcon;
  color?: string;
  size?: string;
}

export type LinkTarget = '_blank' | '_self' | '_parent' | '_top';

export type NavItemTypeObject = { children?: NavItemType[]; items?: NavItemType[]; type?: string };

export type NavItemType = {
  id?: string;
  icon?: GenericCardProps['iconPrimary'];
  target?: boolean;
  external?: boolean;
  url?: string | undefined;
  type?: string;
  title?: ReactNode | string;
  color?: 'primary' | 'secondary' | 'default' | undefined;
  caption?: ReactNode | string;
  breadcrumbs?: boolean;
  disabled?: boolean;
  chip?: ChipProps;
  children?: NavItemType[];
  elements?: NavItemType[];
  search?: string;
};
