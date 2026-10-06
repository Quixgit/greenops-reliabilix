import * as echarts from "echarts/core";
import { LineChart, PieChart, MapChart } from "echarts/charts";
import { GridComponent, TooltipComponent, LegendComponent, VisualMapComponent, GeoComponent, TitleComponent } from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
import { feature } from "topojson-client";
import type { Topology } from "topojson-specification";
import world from "world-atlas/countries-110m.json";

echarts.use([LineChart, PieChart, MapChart, GridComponent, TooltipComponent, LegendComponent, VisualMapComponent, GeoComponent, TitleComponent, CanvasRenderer]);

let registered = false;
/** Registers the world map once (Natural Earth 110m countries via world-atlas). */
export function ensureWorldMap() {
  if (registered) return;
  const topo = world as unknown as Topology;
  const fc = feature(topo, topo.objects.countries as never) as unknown as Parameters<typeof echarts.registerMap>[1];
  echarts.registerMap("world", fc);
  registered = true;
}

export { echarts };
