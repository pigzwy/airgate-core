package store

import (
	"testing"
)

func TestPercentile(t *testing.T) {
	cases := []struct {
		name   string
		sorted []int64
		p      float64
		want   int64
	}{
		{"空切片", nil, 0.5, 0},
		{"单元素", []int64{7}, 0.99, 7},
		{"p50", []int64{1, 2, 3, 4, 5}, 0.50, 3},
		// nearest-rank：idx=int(p*(n-1))，p99 of 5 → idx int(3.96)=3 → 4
		{"p99 近似", []int64{1, 2, 3, 4, 5}, 0.99, 4},
		{"p100 取末位", []int64{1, 2, 3, 4, 5}, 1.0, 5},
		{"p0 取首位", []int64{10, 20, 30}, 0.0, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := percentile(c.sorted, c.p); got != c.want {
				t.Errorf("percentile(%v,%v)=%d, want %d", c.sorted, c.p, got, c.want)
			}
		})
	}
}

func TestComputePercentiles(t *testing.T) {
	// 未排序输入，应内部排序后计算
	got := computePercentiles([]int64{5, 1, 3, 2, 4})
	if got.Samples != 5 {
		t.Errorf("Samples=%d, want 5", got.Samples)
	}
	if got.Max != 5 {
		t.Errorf("Max=%d, want 5", got.Max)
	}
	if got.Avg != 3 {
		t.Errorf("Avg=%v, want 3", got.Avg)
	}
	if got.P50 != 3 {
		t.Errorf("P50=%d, want 3", got.P50)
	}
	// 空输入返回零值
	empty := computePercentiles(nil)
	if empty.Samples != 0 || empty.Max != 0 {
		t.Errorf("空输入应零值，实际 %+v", empty)
	}
}

func TestBuildErrorClasses(t *testing.T) {
	// upstream=6, client=3, gateway=1, other=0, total=10
	got := buildErrorClasses(6, 3, 1, 0, 10)
	if len(got) != 3 {
		t.Fatalf("应只列非零的 3 类，实际 %d: %+v", len(got), got)
	}
	// 第一类 upstream_5xx，比例 0.6
	if got[0].Kind != "upstream_5xx" || got[0].Count != 6 {
		t.Errorf("首类应 upstream_5xx=6，实际 %+v", got[0])
	}
	if got[0].Ratio != 0.6 {
		t.Errorf("upstream 比例应 0.6，实际 %v", got[0].Ratio)
	}
	// total=0 不应除零
	z := buildErrorClasses(0, 0, 0, 0, 0)
	if len(z) != 0 {
		t.Errorf("全零应空，实际 %+v", z)
	}
}
