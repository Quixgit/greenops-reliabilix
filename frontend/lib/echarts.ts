import * as echarts from "echarts/core";
import { LineChart, PieChart, ScatterChart } from "echarts/charts";
import { GridComponent, TooltipComponent, LegendComponent, TitleComponent, MarkLineComponent } from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";

echarts.use([LineChart, PieChart, ScatterChart, GridComponent, TooltipComponent, LegendComponent, TitleComponent, MarkLineComponent, CanvasRenderer]);

export { echarts };
