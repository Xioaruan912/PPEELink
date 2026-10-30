export interface CompatibilityItem {
  id: number;
  name: string;
  protocol: string;
  status: "supported" | "warning" | "unsupported";
  reason: string;
}
export interface ClientCompatibility {
  key: string;
  name: string;
  note: string;
  total: number;
  included: number;
  warnings: number;
  excluded: number;
  items: CompatibilityItem[];
}
export interface CompatibilityData {
  clients: ClientCompatibility[];
  limited?: boolean;
  revision?: string;
  note?: string;
}
