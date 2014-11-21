package types

type Resource struct {
	// Milli cores.
	// TODO(vishh): Rename to MilliCpu
	Cpu    uint64
	Memory uint64
}
