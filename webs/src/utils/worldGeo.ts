export interface CountryFeature {
  name: string;
  d: string;
}

let cachedPromise: Promise<CountryFeature[]> | null = null;

function buildPath(geometry: {
  type: string;
  coordinates: number[][][] | number[][][][];
}): string {
  const polygons =
    geometry.type === "Polygon"
      ? [geometry.coordinates as number[][][]]
      : geometry.type === "MultiPolygon"
        ? (geometry.coordinates as number[][][][])
        : [];

  return polygons
    .flatMap((poly) =>
      poly.map(
        (ring) =>
          ring
            .map(
              ([lng, lat], i) =>
                `${i ? "L" : "M"}${(((lng + 180) / 360) * 1000).toFixed(1)},${(((90 - lat) / 180) * 500).toFixed(1)}`
            )
            .join("") + "Z"
      )
    )
    .join("");
}

export function loadWorldCountries(): Promise<CountryFeature[]> {
  if (!cachedPromise) {
    cachedPromise = fetch(`${import.meta.env.BASE_URL}world.json`)
      .then((res) => {
        if (!res.ok) throw new Error("Failed to load world.json");
        return res.json();
      })
      .then((world) =>
        (world.features || [])
          .filter((f: any) => f?.geometry && f?.properties?.name !== "Antarctica")
          .map((f: any) => ({
            name: String(f.properties?.name || ""),
            d: buildPath(f.geometry),
          }))
          .filter((c: CountryFeature) => c.d.length > 0)
      )
      .catch((err) => {
        cachedPromise = null;
        throw err;
      });
  }
  return cachedPromise;
}
