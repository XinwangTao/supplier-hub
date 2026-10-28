package catalog

import (
	"sort"
	"strings"

	"github.com/example/supplier-hub/backend/internal/domain"
)

// PartCategories 部件小类字典。跨产品复用，不允许各产品自造词，
// 否则"这一类料谁能供"就聚合不起来。
var PartCategories = []domain.PartCategory{
	{Code: "soc", Name: "主控与存储", Description: "SoC / MCU / 内存 / 闪存 / 时钟", KeyCapabilities: []string{"pcba", "testing"}},
	{Code: "power", Name: "电源与电池", Description: "电芯、BMS、充电与电源管理、电量计", KeyCapabilities: []string{"battery", "pcba", "charger"}},
	{Code: "passive", Name: "被动元件", Description: "电阻、电容、电感、共模器件", KeyCapabilities: []string{"pcba"}},
	{Code: "connector", Name: "连接器与线束", Description: "板对板、Type-C、端子、线束组件", KeyCapabilities: []string{"connector", "stamping", "injection"}},
	{Code: "rf", Name: "射频与天线", Description: "蜂窝 / WiFi / GNSS 天线、射频前端、NFC 线圈", KeyCapabilities: []string{"antenna", "fpc", "pcba"}},
	{Code: "optics", Name: "摄像与光学", Description: "图像传感器、镜头、VCM、红外投射", KeyCapabilities: []string{"camera", "lens", "coating"}},
	{Code: "display", Name: "显示与触控", Description: "LCD / OLED 面板、触控层、显示驱动、盖板玻璃", KeyCapabilities: []string{"display", "glass", "coating"}},
	{Code: "acoustics", Name: "声学器件", Description: "扬声器、受话器、MEMS 麦克风、振膜", KeyCapabilities: []string{"acoustics", "injection"}},
	{Code: "sensor", Name: "传感与 MEMS", Description: "IMU、光学心率、气压、温湿度、激光雷达", KeyCapabilities: []string{"sensor", "pcba", "lens"}},
	{Code: "motor", Name: "电机与驱动", Description: "无刷 / 步进 / 空心杯电机、驱动器、编码器", KeyCapabilities: []string{"motor", "magnet", "pcba"}},
	{Code: "transmission", Name: "传动与轴承", Description: "减速器、齿轮、丝杠、导轨、轴承、联轴器", KeyCapabilities: []string{"cnc", "mold", "mim"}},
	{Code: "structure", Name: "结构件与外壳", Description: "压铸 / 冲压 / 型材 / 注塑外壳、装饰件", KeyCapabilities: []string{"diecasting", "stamping", "injection", "cnc", "anodizing"}},
	{Code: "seal", Name: "密封与紧固", Description: "O 型圈、油封、防水膜、螺钉螺母", KeyCapabilities: []string{"silicone", "fastener", "stamping"}},
	{Code: "thermal", Name: "热管理", Description: "均热板、导热垫、散热鳍片、风扇", KeyCapabilities: []string{"heatsink", "diecasting", "stamping"}},
	{Code: "pcb", Name: "PCB 与基板", Description: "刚性板、FPC、软硬结合板、陶瓷基板", KeyCapabilities: []string{"pcb", "fpc"}},
	{Code: "cable", Name: "线缆与屏蔽", Description: "拖链电缆、同轴、屏蔽罩、编织网", KeyCapabilities: []string{"connector", "stamping", "laser"}},
	{Code: "software", Name: "软件与固件", Description: "固件、驱动、算法、上位机（非物理件）", KeyCapabilities: []string{"assembly", "testing"}},
	{Code: "packaging", Name: "包装与辅料", Description: "彩盒、吸塑、胶粘剂、标签", KeyCapabilities: []string{"packaging", "coating"}},
	// module 是"可直接采购的功能单元"：遥控器、电源适配器、成品模组。
	// 单独设一类是因为它们的采购对象不是某个工艺，而是一家能做整件的方案商，
	// 硬塞进 PCB 或结构件会让那两类的供应覆盖统计失真。
	{Code: "module", Name: "成品模组与附件", Description: "遥控器、电源适配器、功能模块等可直接采购的整件", KeyCapabilities: []string{"assembly", "testing", "pcba"}},
}

func CategoryByCode(code string) (domain.PartCategory, bool) {
	for _, c := range PartCategories {
		if c.Code == code {
			return c, true
		}
	}
	return domain.PartCategory{}, false
}

// part 构造部件的快捷方式，自动补小类名称，避免手写时拼错类名。
func part(code, name, category, qty, capability string, critical bool, note string) domain.BomPart {
	catName := ""
	if c, ok := CategoryByCode(category); ok {
		catName = c.Name
	}
	return domain.BomPart{
		Code: code, Name: name, Category: category, CategoryName: catName,
		Qty: qty, Critical: critical, Capability: capability, Note: note,
	}
}

func group(code, name, role string, critical bool, parts ...domain.BomPart) domain.BomGroup {
	return domain.BomGroup{Code: code, Name: name, Role: role, Parts: parts, Critical: critical}
}

// BomProducts 产品 BOM 库。部件结构整理自公开拆解资料（bomwiki 等公开 BOM 目录
// 与厂商拆解报告），用于寻源下钻，不作为任何具体机型的官方 BOM。
var BomProducts = []domain.BomProduct{
	smartphoneBOM(),
	smartwatchBOM(),
	twsBOM(),
	speakerBOM(),
	powerBankBOM(),
	robotVacuumBOM(),
	droneBOM(),
	smartLockBOM(),
	eBikeBOM(),
	airConditionerBOM(),
	robotArmBOM(),
	servoJointBOM(),
	cncMachineBOM(),
	plcCabinetBOM(),
}

// 组装完成后统一算派生字段：部件数、去重能力、关键模块标记。
func init() {
	for i := range BomProducts {
		p := &BomProducts[i]
		caps := map[string]bool{}
		count := 0
		for gi := range p.Groups {
			g := &p.Groups[gi]
			crit := 0
			for _, pt := range g.Parts {
				count++
				if pt.Capability != "" {
					caps[pt.Capability] = true
				}
				if pt.Critical {
					crit++
				}
			}
			if crit >= 2 {
				g.Critical = true
			}
		}
		p.PartCount = count
		p.Capabilities = make([]string, 0, len(caps))
		for c := range caps {
			p.Capabilities = append(p.Capabilities, c)
		}
		sort.Strings(p.Capabilities)
	}
}

func BomProductByCode(code string) (domain.BomProduct, bool) {
	code = strings.ToLower(strings.TrimSpace(code))
	for _, p := range BomProducts {
		if p.Code == code {
			return p, true
		}
	}
	return domain.BomProduct{}, false
}

// MatchBomProduct 按产品名/别名做包含匹配，供"我想做 X"直接下钻到 BOM。
func MatchBomProduct(query string) (domain.BomProduct, bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return domain.BomProduct{}, false
	}
	// 去掉口语前缀，避免"我想做一个智能手表"匹配不上
	for _, prefix := range []string{"我想做", "我想做一款", "我想做一台", "我要做", "做一个", "做一款", "开发一款", "开发一个", "想做"} {
		q = strings.TrimPrefix(q, prefix)
	}
	q = strings.TrimSpace(q)
	best := domain.BomProduct{}
	bestLen := 0
	for _, p := range BomProducts {
		names := append([]string{p.Name}, p.Aliases...)
		for _, n := range names {
			ln := strings.ToLower(n)
			if strings.Contains(q, ln) || strings.Contains(ln, q) {
				if len(ln) > bestLen {
					best, bestLen = p, len(ln)
				}
			}
		}
	}
	if bestLen == 0 {
		return domain.BomProduct{}, false
	}
	return best, true
}

// BomFacets 按小类聚合整个 BOM 库，回答"这类料一共被多少产品用到"。
func BomFacets() []domain.BomFacet {
	parts := map[string]int{}
	critical := map[string]int{}
	products := map[string]map[string]bool{}
	for _, p := range BomProducts {
		for _, g := range p.Groups {
			for _, pt := range g.Parts {
				parts[pt.Category]++
				if pt.Critical {
					critical[pt.Category]++
				}
				if products[pt.Category] == nil {
					products[pt.Category] = map[string]bool{}
				}
				products[pt.Category][p.Name] = true
			}
		}
	}
	out := make([]domain.BomFacet, 0, len(PartCategories))
	for _, c := range PartCategories {
		list := make([]string, 0, len(products[c.Code]))
		for name := range products[c.Code] {
			list = append(list, name)
		}
		sort.Strings(list)
		out = append(out, domain.BomFacet{
			Category: c, Parts: parts[c.Code], Products: list, Critical: critical[c.Code],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Parts > out[j].Parts })
	return out
}

// BomSectors 产品板块分组，前端按板块展示。
func BomSectors() map[string][]string {
	out := map[string][]string{}
	for _, p := range BomProducts {
		out[p.Sector] = append(out[p.Sector], p.Name)
	}
	return out
}
