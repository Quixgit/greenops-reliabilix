import * as echarts from "echarts/core";
import { LineChart, PieChart, MapChart } from "echarts/charts";
import { GridComponent, TooltipComponent, LegendComponent, VisualMapComponent, GeoComponent, TitleComponent } from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
import { feature } from "topojson-client";
import type { Topology } from "topojson-specification";
import world from "world-atlas/countries-110m.json";
import { cleanWorld } from "./geo";

echarts.use([LineChart, PieChart, MapChart, GridComponent, TooltipComponent, LegendComponent, VisualMapComponent, GeoComponent, TitleComponent, CanvasRenderer]);

let registered = false;
/** Registers the world map once (Natural Earth 110m countries via world-atlas). */
export function ensureWorldMap() {
  if (registered) return;
  const topo = world as unknown as Topology;
  const raw = feature(topo, topo.objects.countries as never) as unknown as Parameters<typeof cleanWorld>[0];
  echarts.registerMap("world", cleanWorld(raw) as unknown as Parameters<typeof echarts.registerMap>[1]);
  registered = true;
}

export { echarts };
