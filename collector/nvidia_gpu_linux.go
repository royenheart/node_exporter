// Copyright 2016 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build nvgpu
// +build nvgpu

package collector

import (
	"fmt"
	"strconv"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

type nvgpuCollector struct {
	gpuSysInfo           *prometheus.Desc
	gpuInfo              *prometheus.Desc
	gpuMinFanSpeed       *prometheus.Desc
	gpuMaxFanSpeed       *prometheus.Desc
	gpuFanSpeed          *prometheus.Desc
	gpuTemp              *prometheus.Desc
	gpuPowerUsage        *prometheus.Desc
	gpuPowerEnforceLimit *prometheus.Desc
	gpuMemTotal          *prometheus.Desc
	gpuMemUsed           *prometheus.Desc
	gpuMemFree           *prometheus.Desc
	gpuAppClk            *prometheus.Desc
	gpuClk               *prometheus.Desc
	gpuComputeMode       *prometheus.Desc
	gpuPerf              *prometheus.Desc
	gpuPersisMode        *prometheus.Desc
	gpuUtil              *prometheus.Desc
	logger               log.Logger
}

const (
	nvgpuCollectorSubsystem = "nv" + gpuCollectorSubsystem
)

var (
	enableNVGPUSysInfo = kingpin.Flag("collector.nvgpu.sysinfo", "Enable metric nvgpu system info").Default("false").Bool()
	enableNVGPUInfo    = kingpin.Flag("collector.nvgpu.gpuinfo", "Enable metric nvgpu gpu info").Default("false").Bool()
	enableNVGPUFan     = kingpin.Flag("collector.nvgpu.faninfo", "Enable metric nvgpu fan info").Default("false").Bool()
)

func init() {
	registerCollector("nvgpu", defaultDisabled, NewNVGPUCollector)
}

func NewNVGPUCollector(logger log.Logger) (Collector, error) {
	c := &nvgpuCollector{
		gpuSysInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "sysinfo"),
			"System information from nvml.",
			[]string{"driver_v", "cuda_v", "nvml_v"}, nil,
		),
		gpuInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "gpuinfo"),
			"GPU information from nvml.",
			[]string{"index", "uuid", "name", "bus_type"}, nil,
		),
		gpuAppClk: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "appclk"),
			"GPU Applications Clock information from nvml.  (MHz)",
			[]string{"index", "type"}, nil,
		),
		gpuClk: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "clk"),
			"GPU Clock information from nvml. (MHz)",
			[]string{"index", "type", "id"}, nil,
		),
		gpuComputeMode: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "compute_mode"),
			"GPU Compute Mode information from nvml.",
			[]string{"index", "mode"}, nil,
		),
		gpuPerf: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "perf"),
			"GPU Performance State information from nvml.",
			[]string{"index", "state"}, nil,
		),
		gpuPersisMode: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "persis_mode"),
			"GPU Persistence Mode information from nvml.",
			[]string{"index", "mode"}, nil,
		),
		gpuUtil: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "util"),
			"GPU Utilization Rates information from nvml. (percentage)",
			[]string{"index", "type"}, nil,
		),
		gpuMinFanSpeed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "min_fan_speed"),
			"GPU Min Fan Speed from nvml. (rpm)",
			[]string{"index"}, nil,
		),
		gpuMaxFanSpeed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "max_fan_speed"),
			"GPU Max Fan Speed from nvml. (rpm)",
			[]string{"index"}, nil,
		),
		gpuFanSpeed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "fan_speed"),
			"GPU Fan Speed from nvml. It's the percentage of the maximum fan speed, which may exceed 100%",
			[]string{"index", "fan"}, nil,
		),
		gpuTemp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "temp"),
			"GPU Temperature information from nvml in Celsius format.",
			[]string{"index", "type"}, nil,
		),
		gpuPowerUsage: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "power_usage"),
			"GPU Power Usage information from nvml (milliwatt).",
			[]string{"index"}, nil,
		),
		gpuPowerEnforceLimit: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "power_enforce_limit"),
			"GPU Power Enforced Limitation information from nvml (milliwatt).",
			[]string{"index"}, nil,
		),
		gpuMemTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "mem_total"),
			"GPU Memory Total from nvml (bytes IEC).",
			[]string{"index"}, nil,
		),
		gpuMemUsed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "mem_used"),
			"GPU Memory Used from nvml (bytes IEC).",
			[]string{"index"}, nil,
		),
		gpuMemFree: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, nvgpuCollectorSubsystem, "mem_free"),
			"GPU Memory Free from nvml (bytes IEC).",
			[]string{"index"}, nil,
		),
		logger: logger,
	}
	return c, nil
}

// Update implements Collector.
func (c *nvgpuCollector) Update(ch chan<- prometheus.Metric) error {
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("unable to initialize NVML: %v", nvml.ErrorString(ret))
	}
	defer func() {
		ret := nvml.Shutdown()
		if ret != nvml.SUCCESS {
			level.Error(c.logger).Log("msg", fmt.Sprintf("unable to shutdown NVML: %v", nvml.ErrorString(ret)))
		}
	}()

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("unable to get device count: %v", nvml.ErrorString(ret))
	}

	var devices []nvml.Device
	for i := 0; i < count; i++ {
		d, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			return fmt.Errorf("unable to get device at index %d: %v", i, nvml.ErrorString(ret))
		}
		// it make sures that the index of the array response to the GPU Index
		devices = append(devices, d)
	}

	if *enableNVGPUSysInfo {
		driverV, ret := nvml.SystemGetDriverVersion()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("unable to Get NVIDIA System Driver Version: %v", nvml.ErrorString(ret))
		}
		/*
			SystemGetCudaDriverVersion_v2 calls c func nvmlSystemGetCudaDriverVersion() from nvml, and it retrieves version from the shared library, the returned version by calling c func cudaDriverGetVersion() from cuda, the version is return as int with exp (1000 * major + 10 * minor), For example, 11.7 is retrieved as 11070
		*/
		cudaVI, ret := nvml.SystemGetCudaDriverVersion_v2()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("unable to Get NVIDIA CUDA Driver Version: %v", nvml.ErrorString(ret))
		}
		cudaV := fmt.Sprintf("%d.%d", cudaVI/1000, cudaVI%1000/10)
		nvmlV, ret := nvml.SystemGetNVMLVersion()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("unable to Get NVIDIA NVML Version: %v", nvml.ErrorString(ret))
		}

		ch <- prometheus.MustNewConstMetric(
			c.gpuSysInfo,
			prometheus.GaugeValue,
			1,
			driverV,
			cudaV,
			nvmlV,
		)
	}

	for indexI, d := range devices {
		index := strconv.Itoa(indexI)

		// Get GPU Info

		if *enableNVGPUInfo {
			uuid, ret := d.GetUUID()
			if ret != nvml.SUCCESS {
				return fmt.Errorf("unable to get GPU %v UUID: %v", index, nvml.ErrorString(ret))
			}
			name, ret := d.GetName()
			if ret != nvml.SUCCESS {
				return fmt.Errorf("unable to get GPU %v NAME: %v", index, nvml.ErrorString(ret))
			}
			busTypeE, ret := d.GetBusType()
			if ret != nvml.SUCCESS {
				return fmt.Errorf("unable to get GPU %v BUS Type: %v", index, nvml.ErrorString(ret))
			}
			busType := getBusTypeString(busTypeE)

			ch <- prometheus.MustNewConstMetric(
				c.gpuInfo,
				prometheus.GaugeValue,
				1,
				index,
				uuid,
				name,
				busType,
			)
		}

		// Get Fan Info

		if *enableNVGPUFan {
			fanNums, ret := d.GetNumFans()
			if ret != nvml.SUCCESS {
				return fmt.Errorf("unable to get GPU %v Fan info: %v", index, nvml.ErrorString(ret))
			} else {
				if fanNums > 0 {
					minFanSpeed, maxFanSpeed, ret := d.GetMinMaxFanSpeed()
					if ret != nvml.SUCCESS {
						return fmt.Errorf("unable to get GPU %v Fan Min/Max Speed: %v", index, nvml.ErrorString(ret))
					}
					for f := 0; f < fanNums; f++ {
						fanSpeed, ret := d.GetFanSpeed_v2(f)
						if ret != nvml.SUCCESS {
							return fmt.Errorf("unable to get GPU %v Fan %d Speed: %v", index, f, nvml.ErrorString(ret))
						}
						ch <- prometheus.MustNewConstMetric(
							c.gpuMinFanSpeed,
							prometheus.GaugeValue,
							float64(minFanSpeed),
							index,
						)
						ch <- prometheus.MustNewConstMetric(
							c.gpuMaxFanSpeed,
							prometheus.GaugeValue,
							float64(maxFanSpeed),
							index,
						)
						ch <- prometheus.MustNewConstMetric(
							c.gpuFanSpeed,
							prometheus.GaugeValue,
							float64(fanSpeed),
							index,
							strconv.Itoa(f),
						)
					}
				} else {
					level.Debug(c.logger).Log("msg", fmt.Sprintf("GPU %v has not Fan", index))
				}
			}
		}

		// Get Clock Info

		for t := 0; t < int(nvml.CLOCK_COUNT); t++ {
			appclk, ret := d.GetApplicationsClock(nvml.ClockType(t))
			switch ret {
			case nvml.ERROR_NOT_SUPPORTED:
				level.Debug(c.logger).Log("msg", fmt.Sprintf("GPU %v not support App Clock Type %v", index, getClockTypeString(t)))
			case nvml.SUCCESS:
				ch <- prometheus.MustNewConstMetric(
					c.gpuAppClk,
					prometheus.GaugeValue,
					float64(appclk),
					index,
					getClockTypeString(t),
				)
			default:
				return fmt.Errorf("unable to get GPU %v Applications Clock Type %v Info: %v", index, getClockTypeString(t), nvml.ErrorString(ret))
			}
			for tt := 0; tt < int(nvml.CLOCK_ID_COUNT); tt++ {
				clk, ret := d.GetClock(nvml.ClockType(t), nvml.ClockId(tt))
				switch ret {
				case nvml.ERROR_NOT_SUPPORTED:
					level.Debug(c.logger).Log("msg", fmt.Sprintf("GPU %v not support Clock Type %v with Clock ID %v", index, getClockTypeString(t), getClockIDString(tt)))
				case nvml.SUCCESS:
					ch <- prometheus.MustNewConstMetric(
						c.gpuClk,
						prometheus.GaugeValue,
						float64(clk),
						index,
						getClockTypeString(t),
						getClockIDString(tt),
					)
				default:
					return fmt.Errorf("unable to get GPU %v Clock Type %v ID %v Info: %v", index, getClockTypeString(t), getClockIDString(tt), nvml.ErrorString(ret))
				}
			}
		}

		// Get Compute Mode

		compute_m, ret := d.GetComputeMode()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("unable to get GPU %v Compute Mode Info: %v", index, nvml.ErrorString(ret))
		}
		ch <- prometheus.MustNewConstMetric(
			c.gpuComputeMode,
			prometheus.GaugeValue,
			1,
			index,
			getComputeModeString(compute_m),
		)

		// Get Performance State

		perf_state, ret := d.GetPerformanceState()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("unable to get GPU %v Performance State Info: %v", index, nvml.ErrorString(ret))
		}
		ch <- prometheus.MustNewConstMetric(
			c.gpuPerf,
			prometheus.GaugeValue,
			1,
			index,
			getPstatesString(perf_state),
		)

		// Get Persistence Mode

		persis_mode, ret := d.GetPersistenceMode()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("unable to get GPU %v Persistence Mode Info: %v", index, nvml.ErrorString(ret))
		}
		ch <- prometheus.MustNewConstMetric(
			c.gpuPersisMode,
			prometheus.GaugeValue,
			1,
			index,
			getPersisModeString(persis_mode),
		)

		// Get GPU Utilization

		util, ret := d.GetUtilizationRates()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("unable to get GPU %v Utilization Info: %v", index, nvml.ErrorString(ret))
		}
		ch <- prometheus.MustNewConstMetric(
			c.gpuUtil,
			prometheus.GaugeValue,
			float64(util.Gpu),
			index,
			"GPU",
		)
		ch <- prometheus.MustNewConstMetric(
			c.gpuUtil,
			prometheus.GaugeValue,
			float64(util.Memory),
			index,
			"MEMORY",
		)

		// Get Temperature Info

		for t := 0; t < int(nvml.TEMPERATURE_COUNT); t++ {
			temp, ret := d.GetTemperature(nvml.TemperatureSensors(t))
			if ret != nvml.SUCCESS {
				return fmt.Errorf("unable to get GPU %v Temperature Sensor %v Value: %v", index, getTemperatureSensorString(t), nvml.ErrorString(ret))
			}
			ch <- prometheus.MustNewConstMetric(
				c.gpuTemp,
				prometheus.GaugeValue,
				float64(temp),
				index,
				getTemperatureSensorString(t),
			)
		}

		// Get Power Info

		power, ret := d.GetPowerUsage()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("unable to get GPU %v Power Usage Value: %v", index, nvml.ErrorString(ret))
		}
		ch <- prometheus.MustNewConstMetric(
			c.gpuPowerUsage,
			prometheus.GaugeValue,
			float64(power),
			index,
		)
		enforce_limit, ret := d.GetEnforcedPowerLimit()
		switch ret {
		case nvml.ERROR_NOT_SUPPORTED:
			level.Debug(c.logger).Log("msg", fmt.Sprintf("GPU %v not support Enforced Power Limitation", index))
		case nvml.SUCCESS:
			ch <- prometheus.MustNewConstMetric(
				c.gpuPowerEnforceLimit,
				prometheus.GaugeValue,
				float64(enforce_limit),
				index,
			)
		default:
			return fmt.Errorf("unable to get GPU %v Power Enforced Limitation Value: %v", index, nvml.ErrorString(ret))
		}

		/*
			Get Memory Info
			GetMemoryInfo_v2() could not correctly show the memory info in some situation.
		*/

		mem, ret := d.GetMemoryInfo()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("unable to get GPU %v Memory Info: %v", index, nvml.ErrorString(ret))
		}
		ch <- prometheus.MustNewConstMetric(
			c.gpuMemTotal,
			prometheus.GaugeValue,
			float64(mem.Total),
			index,
		)
		ch <- prometheus.MustNewConstMetric(
			c.gpuMemFree,
			prometheus.GaugeValue,
			float64(mem.Free),
			index,
		)
		ch <- prometheus.MustNewConstMetric(
			c.gpuMemUsed,
			prometheus.GaugeValue,
			float64(mem.Used),
			index,
		)
	}

	return nil
}

func getBusTypeString(busT nvml.BusType) string {
	switch busT {
	case nvml.BUS_TYPE_AGP:
		return "AGP"
	case nvml.BUS_TYPE_FPCI:
		return "FPCI"
	case nvml.BUS_TYPE_PCI:
		return "PCI"
	case nvml.BUS_TYPE_PCIE:
		return "PCIE"
	case nvml.BUS_TYPE_UNKNOWN:
		return "UNKNOWN"
	default:
		return "UNKNOWN"
	}
}

func getClockTypeString[T int | int32 | nvml.ClockType](typeN T) string {
	switch typeN {
	case T(nvml.CLOCK_GRAPHICS):
		return "GRAPHICS"
	case T(nvml.CLOCK_SM):
		return "SM"
	case T(nvml.CLOCK_MEM):
		return "MEM"
	case T(nvml.CLOCK_VIDEO):
		return "VIDEO"
	default:
		return "UNKNOWN"
	}
}

func getClockIDString[T int | int32 | nvml.ClockId](typeN T) string {
	switch typeN {
	case T(nvml.CLOCK_ID_CURRENT):
		return "CURRENT"
	case T(nvml.CLOCK_ID_APP_CLOCK_TARGET):
		return "APP CLOCK TARGET"
	case T(nvml.CLOCK_ID_APP_CLOCK_DEFAULT):
		return "APP CLOCK DEFAULT"
	case T(nvml.CLOCK_ID_CUSTOMER_BOOST_MAX):
		return "CUSTOMER BOOST MAX"
	default:
		return "UNKNOWN"
	}
}

func getComputeModeString[T int | int32 | nvml.ComputeMode](modeN T) string {
	switch modeN {
	case T(nvml.COMPUTEMODE_DEFAULT):
		return "DEFAULT"
	case T(nvml.COMPUTEMODE_EXCLUSIVE_THREAD):
		return "EXCLUSIVE THREAD"
	case T(nvml.COMPUTEMODE_PROHIBITED):
		return "PROHIBITED"
	case T(nvml.COMPUTEMODE_EXCLUSIVE_PROCESS):
		return "EXCLUSIVE PROCESS"
	default:
		return "UNKNOWN"
	}
}

func getPstatesString[T int | int32 | nvml.Pstates](stateN T) string {
	if nvml.PSTATE_UNKNOWN != nvml.Pstates(stateN) {
		return fmt.Sprintf("P%d", int(stateN))
	} else {
		return "UNKNOWN"
	}
}

func getPersisModeString[T int | int32 | nvml.EnableState](stateN T) string {
	if nvml.EnableState(stateN) == nvml.FEATURE_DISABLED {
		return "OFF"
	} else {
		return "ON"
	}
}

func getTemperatureSensorString[T int | int32 | nvml.TemperatureSensors](tempN T) string {
	switch tempN {
	case T(nvml.TEMPERATURE_GPU):
		return "GPU"
	default:
		return "UNKNOWN"
	}
}
