package ec2spec

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Spec
		ok   bool
	}{
		{"m5.large", Spec{2, 8}, true},
		{"m5.xlarge", Spec{4, 16}, true},
		{"c7g.2xlarge", Spec{8, 16}, true},
		{"r6id.4xlarge", Spec{16, 128}, true},
		{"c7gn.16xlarge", Spec{64, 128}, true},
		{"t3.micro", Spec{2, 1}, true},
		{"t2.micro", Spec{1, 1}, true},
		{"t4g.2xlarge", Spec{8, 32}, true},
		{" M5.Large ", Spec{2, 8}, true}, // input is normalised
		{"m5.metal", Spec{}, false},
		{"p4d.24xlarge", Spec{}, false}, // GPU families are not modelled
		{"u-6tb1.112xlarge", Spec{}, false},
		{"c7i-flex.large", Spec{}, false},
		{"m5.1xlarge", Spec{}, false},
		{"m5.medium", Spec{}, false},
		{"m5.999xlarge", Spec{}, false},
		{"m5", Spec{}, false},
		{"", Spec{}, false},
	}
	for _, c := range cases {
		got, ok := Parse(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}
