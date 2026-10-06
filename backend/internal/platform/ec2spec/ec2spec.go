// Package ec2spec derives vCPU and memory of an EC2 instance type from its name, so that billed
// instance-hours can be converted into measured vCPU-hours and GB-hours (usage-based carbon).
//
// The rules cover the general-purpose, compute, memory and burstable families, whose shape is a pure
// function of the family and size. Families whose size varies per model (GPU, accelerators, bare metal,
// high-memory) are deliberately unsupported: Parse reports ok=false and the caller falls back to the
// cost-based estimate instead of guessing.
package ec2spec

import (
	"strconv"
	"strings"
)

// Spec is the hardware shape of one instance type.
type Spec struct {
	VCPU     float64
	MemoryGB float64
}

// burstable lists the fixed shapes of the t family (they do not follow a GB-per-vCPU ratio).
var burstable = map[string]Spec{
	"nano": {2, 0.5}, "micro": {2, 1}, "small": {2, 2}, "medium": {2, 4}, "large": {2, 8},
	"xlarge": {4, 16}, "2xlarge": {8, 32},
}

// t2 is the one burstable generation whose small sizes have a single vCPU.
var burstableT2 = map[string]Spec{
	"nano": {1, 0.5}, "micro": {1, 1}, "small": {1, 2}, "medium": {2, 4}, "large": {2, 8},
	"xlarge": {4, 16}, "2xlarge": {8, 32},
}

// gbPerVCPU is the memory-to-vCPU ratio of each supported class (first letter of the family).
var gbPerVCPU = map[byte]float64{'c': 2, 'm': 4, 'r': 8}

// Parse returns the spec of an instance type such as "m5.large" or "c7g.2xlarge".
func Parse(instanceType string) (Spec, bool) {
	family, size, found := strings.Cut(strings.ToLower(strings.TrimSpace(instanceType)), ".")
	if !found || family == "" || size == "" {
		return Spec{}, false
	}
	if family[0] == 't' {
		table := burstable
		if family == "t2" {
			table = burstableT2
		}
		s, ok := table[size]
		return s, ok
	}
	ratio, ok := gbPerVCPU[family[0]]
	if !ok || !validFamily(family) {
		return Spec{}, false
	}
	vcpu, ok := vcpuForSize(size)
	if !ok {
		return Spec{}, false
	}
	return Spec{VCPU: vcpu, MemoryGB: vcpu * ratio}, true
}

// validFamily accepts "<class><generation><attributes>" (m5, c7gn, r6id) and rejects look-alike families
// whose ratio differs, such as the "-flex" variants (their size table is not the plain ratio).
func validFamily(f string) bool {
	if len(f) < 2 || f[1] < '0' || f[1] > '9' {
		return false
	}
	return !strings.Contains(f, "-")
}

// vcpuForSize maps "large" -> 2, "xlarge" -> 4, "<n>xlarge" -> 4n. Smaller sizes only exist for the
// burstable family and "metal" sizes vary per model, so both are rejected.
func vcpuForSize(size string) (float64, bool) {
	switch size {
	case "large":
		return 2, true
	case "xlarge":
		return 4, true
	}
	n, ok := strings.CutSuffix(size, "xlarge")
	if !ok {
		return 0, false
	}
	k, err := strconv.Atoi(n)
	if err != nil || k < 2 || k > 48 {
		return 0, false
	}
	return float64(4 * k), true
}
