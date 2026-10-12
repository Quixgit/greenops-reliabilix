package domain

import (
	"math"
	"sort"
	"time"
)

// Cost allocation (ADR-0063). What a cluster costs is on the cloud bill (the nodes); who uses it is in the samples (requests and use of every workload,
// per hour). The cost of a day is split between CPU and memory, and each part among the workloads in proportion to what they hold of it: the larger of
// what they request and what they use, because a requested core is not available to anybody else whether or not it is busy. Where the agent also
// reports the nodes' capacity, what nobody holds is **idle**; where it does not, the whole cost is shared out and idle is reported as unknown, never as zero.

// CPUShare is the part of the node cost taken to pay for CPU; the rest pays for memory. The ratio of the list prices of cores and gibibytes in the
// three clouds' general-purpose families is close to 2 to 1 by weight; 65 per cent is the figure used, stated on the report.
const CPUShare = 0.65

// DayWorkload is one workload's day, summed over its hours.
type DayWorkload struct {
	Day                   time.Time
	Namespace, Workload   string
	CPUHold, MemHold      float64 // core-hours and GiB-hours held: per hour the larger of request and use
	CPURequested, CPUUsed float64 // core-hours
	MemRequested, MemUsed float64 // GiB-hours
	CPUKnown, MemKnown    bool    // requests were reported (waste cannot be told without them)
	Hours                 int
}

// DayCluster is one day of a cluster.
type DayCluster struct {
	Day       time.Time
	Pool      float64 // what the day's nodes cost, in the reporting currency
	HoursSeen int     // distinct hours the agent reported workloads in (0 to 24)
	// CapCPU and CapMem are the node capacity summed over the hours that report it (core-hours, GiB-hours); CapHours counts those hours. Zero: unknown.
	CapCPU, CapMem float64
	CapHours       int
}

// WorkloadCost is what one workload cost over the window.
type WorkloadCost struct {
	Namespace, Workload    string
	Cost, CPUCost, MemCost float64
	// Waste is the part of its cost that pays for requests it did not use (zero when it reported no requests).
	Waste                  float64
	CPUUtilPct, MemUtilPct *float64 // use over request, where reported
	Hours                  int
}

// NamespaceCost sums the workloads of a namespace.
type NamespaceCost struct {
	Namespace   string
	Cost, Waste float64
	Workloads   int
}

// Allocation is the result for one cluster and window.
type Allocation struct {
	Pool        float64 // the cost of the hours the agent reported
	Allocated   float64 // given to workloads
	Idle        float64 // held by nobody (only on days with capacity)
	IdleDays    int     // days whose idle is known
	Days        int
	Unallocated float64 // cost that could not be given to anybody (a day with a cost and no workload holding anything)
	Workloads   []WorkloadCost
	Namespaces  []NamespaceCost
}

// IdleKnown says whether idle is known for every day.
func (a Allocation) IdleKnown() bool { return a.Days > 0 && a.IdleDays == a.Days }

// Allocate splits the pool of every day. Workloads are the day-level sums; clusters carries the pool and capacity per day. Days with no cluster row are
// ignored (no pool, nothing to split), days with no workload rows leave their cost unallocated.
func Allocate(clusters []DayCluster, workloads []DayWorkload) Allocation {
	var out Allocation
	byDay := map[time.Time][]DayWorkload{}
	for _, w := range workloads {
		d := w.Day.UTC().Truncate(24 * time.Hour)
		byDay[d] = append(byDay[d], w)
	}
	type acc struct {
		cost, cpu, mem, waste            float64
		cpuReq, cpuUsed, memReq, memUsed float64
		hours                            int
	}
	agg := map[[2]string]*acc{}
	for _, c := range clusters {
		day := c.Day.UTC().Truncate(24 * time.Hour)
		ws := byDay[day]
		covered := c.Pool * math.Min(1, float64(c.HoursSeen)/24)
		if covered <= 0 {
			continue // a day the agent did not report in is not part of this report
		}
		out.Days++
		out.Pool += covered
		var sumCPU, sumMem float64
		for _, w := range ws {
			sumCPU += w.CPUHold
			sumMem += w.MemHold
		}
		var pCPU, pMem float64
		known := c.CapHours > 0 && c.CapCPU > 0 && c.CapMem > 0
		if known {
			// capacity per hour, for the hours the agent reported workloads in (an hour without a capacity figure takes the mean of the others)
			capCPU := c.CapCPU / float64(c.CapHours) * float64(c.HoursSeen)
			capMem := c.CapMem / float64(c.CapHours) * float64(c.HoursSeen)
			pCPU, pMem = covered*CPUShare/capCPU, covered*(1-CPUShare)/capMem
		} else {
			// no capacity: the whole cost is shared among what is held; a part nobody holds goes to the other part
			cpuPool, memPool := covered*CPUShare, covered*(1-CPUShare)
			switch {
			case sumCPU > 0 && sumMem > 0:
			case sumCPU > 0:
				cpuPool, memPool = covered, 0
			case sumMem > 0:
				cpuPool, memPool = 0, covered
			default:
				out.Unallocated += covered
				continue
			}
			if sumCPU > 0 {
				pCPU = cpuPool / sumCPU
			}
			if sumMem > 0 {
				pMem = memPool / sumMem
			}
		}
		var dayTotal float64
		type part struct {
			w        DayWorkload
			cpu, mem float64
		}
		parts := make([]part, 0, len(ws))
		for _, w := range ws {
			p := part{w: w, cpu: w.CPUHold * pCPU, mem: w.MemHold * pMem}
			dayTotal += p.cpu + p.mem
			parts = append(parts, p)
		}
		scale := 1.0
		if dayTotal > covered && dayTotal > 0 { // holds overlap (a request above what the node could give): never allocate more than was paid
			scale = covered / dayTotal
		}
		var given float64
		for _, p := range parts {
			cpu, mem := p.cpu*scale, p.mem*scale
			a := agg[[2]string{p.w.Namespace, p.w.Workload}]
			if a == nil {
				a = &acc{}
				agg[[2]string{p.w.Namespace, p.w.Workload}] = a
			}
			a.cost += cpu + mem
			a.cpu += cpu
			a.mem += mem
			a.hours += p.w.Hours
			if p.w.CPUKnown && p.w.CPURequested > 0 && p.w.CPUHold > 0 {
				a.waste += cpu * math.Max(0, p.w.CPURequested-p.w.CPUUsed) / p.w.CPUHold
				a.cpuReq += p.w.CPURequested
				a.cpuUsed += p.w.CPUUsed
			}
			if p.w.MemKnown && p.w.MemRequested > 0 && p.w.MemHold > 0 {
				a.waste += mem * math.Max(0, p.w.MemRequested-p.w.MemUsed) / p.w.MemHold
				a.memReq += p.w.MemRequested
				a.memUsed += p.w.MemUsed
			}
			given += cpu + mem
		}
		out.Allocated += given
		if known {
			out.Idle += math.Max(0, covered-given)
			out.IdleDays++
		}
	}
	ns := map[string]*NamespaceCost{}
	for k, a := range agg {
		w := WorkloadCost{Namespace: k[0], Workload: k[1], Cost: a.cost, CPUCost: a.cpu, MemCost: a.mem, Waste: a.waste, Hours: a.hours}
		if a.cpuReq > 0 {
			u := a.cpuUsed / a.cpuReq * 100
			w.CPUUtilPct = &u
		}
		if a.memReq > 0 {
			u := a.memUsed / a.memReq * 100
			w.MemUtilPct = &u
		}
		out.Workloads = append(out.Workloads, w)
		n := ns[k[0]]
		if n == nil {
			n = &NamespaceCost{Namespace: k[0]}
			ns[k[0]] = n
		}
		n.Cost += a.cost
		n.Waste += a.waste
		n.Workloads++
	}
	sort.Slice(out.Workloads, func(i, j int) bool {
		a, b := out.Workloads[i], out.Workloads[j]
		if a.Cost != b.Cost {
			return a.Cost > b.Cost
		}
		return a.Namespace+"/"+a.Workload < b.Namespace+"/"+b.Workload
	})
	for _, n := range ns {
		out.Namespaces = append(out.Namespaces, *n)
	}
	sort.Slice(out.Namespaces, func(i, j int) bool {
		if out.Namespaces[i].Cost != out.Namespaces[j].Cost {
			return out.Namespaces[i].Cost > out.Namespaces[j].Cost
		}
		return out.Namespaces[i].Namespace < out.Namespaces[j].Namespace
	})
	return out
}
