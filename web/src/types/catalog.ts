export type CatalogModuleKind = 'terraform' | 'helm';

export interface CatalogModuleVariable {
  name: string;
  description: string;
  type: string;
  required?: boolean;
  default?: string;
}

export interface CatalogModule {
  id: string;
  kind: CatalogModuleKind;
  name: string;
  shortName: string;
  version: string;
  summary: string;
  description: string;
  providerOrChart: string;
  inputs: CatalogModuleVariable[];
  /** Terraform modules expose declared outputs; Helm charts do not use this field. */
  outputs?: CatalogModuleVariable[];
  tags: string[];
}
