package domain

import (
	"math"
	"testing"
	"time"
)

var d1 = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestWithoutCapacityTheWholeCostIsSharedByWhatIsHeld(t *testing.T) {
	a := Allocate([]DayCluster{{Day: d1, Pool: 240, HoursSeen: 24}}, []DayWorkload{
		{Day: d1, Namespace: "shop", Workload: "web", CPUHold: 48, MemHold: 96, Hours: 24},
		{Day: d1, Namespace: "shop", Workload: "db", CPUHold: 24, MemHold: 288, Hours: 24},
		{Day: d1, Namespace: "ops", Workload: "agent", CPUHold: 24, MemHold: 24, Hours: 24},
	})
	if !near(a.Pool, 240) || !near(a.Allocated, 240) || a.Idle != 0 || a.IdleKnown() || a.IdleDays != 0 {
		t.Fatalf("every cent is allocated and idle is unknown: %+v", a)
	}
	var sum float64
	for _, w := range a.Workloads {
		sum += w.Cost
	}
	if !near(sum, 240) {
		t.Fatalf("workloads add up to the pool, got %v", sum)
	}
	// cpu pool 156 over 96 core-hours, memory pool 84 over 408 GiB-hours
	web := find(a, "shop", "web")
	if !near(web.CPUCost, 48*156.0/96) || !near(web.MemCost, 96*84.0/408) {
		t.Fatalf("%+v", web)
	}
	if a.Namespaces[0].Namespace != "shop" || !near(a.Namespaces[0].Cost+a.Namespaces[1].Cost, 240) {
		t.Fatalf("%+v", a.Namespaces)
	}
}

func find(a Allocation, ns, w string) WorkloadCost {
	for _, x := range a.Workloads {
		if x.Namespace == ns && x.Workload == w {
			return x
		}
	}
	return WorkloadCost{}
}

func TestWithCapacityWhatNobodyHoldsIsIdle(t *testing.T) {
	// 10 cores and 40 GiB of nodes for 24 hours cost 240; the workloads hold 4 core-hours x 24 and 8 GiB-hours x 24
	a := Allocate([]DayCluster{{Day: d1, Pool: 240, HoursSeen: 24, CapCPU: 240, CapMem: 960, CapHours: 24}}, []DayWorkload{
		{Day: d1, Namespace: "shop", Workload: "web", CPUHold: 96, MemHold: 192, Hours: 24},
	})
	cpu := 96 * (240 * CPUShare / 240)
	mem := 192 * (240 * (1 - CPUShare) / 960)
	if !a.IdleKnown() || !near(a.Allocated, cpu+mem) || !near(a.Idle, 240-cpu-mem) || !near(a.Allocated+a.Idle, 240) {
		t.Fatalf("%+v (want allocated %v)", a, cpu+mem)
	}
}

func TestOnlyTheHoursTheAgentReportedAreAllocated(t *testing.T) {
	a := Allocate([]DayCluster{{Day: d1, Pool: 240, HoursSeen: 12}}, []DayWorkload{{Day: d1, Namespace: "n", Workload: "w", CPUHold: 12, MemHold: 12, Hours: 12}})
	if !near(a.Pool, 120) || !near(a.Allocated, 120) {
		t.Fatalf("half a day reported is half a day's cost: %+v", a)
	}
}

func TestHoldsAboveCapacityNeverAllocateMoreThanWasPaid(t *testing.T) {
	a := Allocate([]DayCluster{{Day: d1, Pool: 100, HoursSeen: 24, CapCPU: 24, CapMem: 24, CapHours: 24}}, []DayWorkload{
		{Day: d1, Namespace: "n", Workload: "a", CPUHold: 40, MemHold: 40, Hours: 24}})
	if a.Allocated > 100+1e-9 || a.Idle != 0 || !near(a.Allocated, 100) {
		t.Fatalf("%+v", a)
	}
}

func TestAMissingCapacityHourTakesTheMeanOfTheOthers(t *testing.T) {
	// 12 hours with capacity 10 cores each, 24 hours of workloads: the capacity of the day is taken as 24 x 10
	a := Allocate([]DayCluster{{Day: d1, Pool: 240, HoursSeen: 24, CapCPU: 120, CapMem: 480, CapHours: 12}}, []DayWorkload{
		{Day: d1, Namespace: "n", Workload: "w", CPUHold: 120, MemHold: 480, Hours: 24}})
	if !near(a.Allocated, 120*(240*CPUShare/240)+480*(240*(1-CPUShare)/960)) {
		t.Fatalf("%+v", a)
	}
}

func TestWasteIsThePartOfTheCostThatPaysForUnusedRequests(t *testing.T) {
	a := Allocate([]DayCluster{{Day: d1, Pool: 100, HoursSeen: 24}}, []DayWorkload{
		// requests 10 core-hours and uses 2: 80 per cent of its CPU cost is waste; memory is not reported
		{Day: d1, Namespace: "n", Workload: "w", CPUHold: 10, MemHold: 0, CPURequested: 10, CPUUsed: 2, CPUKnown: true, Hours: 24}})
	w := a.Workloads[0]
	if !near(w.Cost, 100) || !near(w.Waste, 80) || w.CPUUtilPct == nil || !near(*w.CPUUtilPct, 20) || w.MemUtilPct != nil {
		t.Fatalf("%+v", w)
	}
	// a workload that uses more than it asks has no waste
	b := Allocate([]DayCluster{{Day: d1, Pool: 100, HoursSeen: 24}}, []DayWorkload{
		{Day: d1, Namespace: "n", Workload: "w", CPUHold: 10, CPURequested: 4, CPUUsed: 10, CPUKnown: true, Hours: 24}})
	if b.Workloads[0].Waste != 0 {
		t.Fatalf("%+v", b.Workloads[0])
	}
}

// Only days the agent reported in count: a day with cost and no report is not costed at all.
func TestADayWithCostAndNoWorkloadIsUnallocatedNotLost(t *testing.T) {
	a := Allocate([]DayCluster{{Day: d1, Pool: 50, HoursSeen: 24}, {Day: d1.AddDate(0, 0, 1), Pool: 10, HoursSeen: 0}}, nil)
	if !near(a.Unallocated, 50) || a.Allocated != 0 || a.Days != 1 || !near(a.Pool, 50) {
		t.Fatalf("%+v", a)
	}
}

func TestAllocationSumsAcrossDaysAndOrdersByCost(t *testing.T) {
	d2 := d1.AddDate(0, 0, 1)
	a := Allocate([]DayCluster{{Day: d1, Pool: 100, HoursSeen: 24}, {Day: d2, Pool: 200, HoursSeen: 24}}, []DayWorkload{
		{Day: d1, Namespace: "a", Workload: "x", CPUHold: 1, MemHold: 1, Hours: 24}, {Day: d2, Namespace: "a", Workload: "x", CPUHold: 1, MemHold: 1, Hours: 24},
		{Day: d2, Namespace: "b", Workload: "y", CPUHold: 1, MemHold: 1, Hours: 24}})
	if !near(a.Allocated, 300) || a.Workloads[0].Workload != "x" || !near(a.Workloads[0].Cost, 100+100) || !near(a.Workloads[1].Cost, 100) || a.Workloads[0].Hours != 48 {
		t.Fatalf("%+v", a.Workloads)
	}
}
