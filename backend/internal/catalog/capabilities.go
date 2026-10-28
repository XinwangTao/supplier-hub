// Package catalog 保存供应链知识底座：工艺能力字典、产品配方（模块 → 工艺 → 能力）、
// 产业带与行政区划信息。它不依赖任何存储，是网络图与寻源检索的共同事实来源。
package catalog

// Capability 是一类可被采购的工艺能力，是网状图里的“行星节点”。
// 供应商通过 Capabilities 字段挂接到一个或多个能力上，产品通过配方引用能力，
// 二者在图上交叉，就形成了可复用的供应链网。
type Capability struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Short       string   `json:"short"`    // 用于企业命名，如「精密注塑」
	Category    string   `json:"category"` // 结构件 / 电子 / 电源 / 组装服务
	Description string   `json:"description"`
	Processes   []string `json:"processes"` // 典型工序，用于展示与检索
	Regions     []string `json:"regions"`   // 主产业带城市
	Pinyin      string   `json:"pinyin"`
}

var Capabilities = []Capability{
	// ---------- 结构件 ----------
	{Code: "cnc", Name: "CNC 精密加工", Short: "精密加工", Category: "结构件",
		Description: "三轴/五轴 CNC、车铣复合，适合中框、外壳、精密小件的小批量与打样",
		Processes:   []string{"五轴联动", "车铣复合", "精铣倒角", "去毛刺"}, Regions: []string{"东莞市", "深圳市", "苏州市"}, Pinyin: "cnc"},
	{Code: "mold", Name: "模具设计制造", Short: "模具", Category: "结构件",
		Description: "塑胶模、压铸模、级进模的设计与制造，决定产品外观与量产一致性",
		Processes:   []string{"模流分析", "慢走丝", "镜面放电", "试模"}, Regions: []string{"东莞市", "苏州市", "宁波市"}, Pinyin: "mold"},
	{Code: "injection", Name: "精密注塑", Short: "精密注塑", Category: "结构件",
		Description: "双色注塑、包胶、纳米注塑与液态硅胶成型",
		Processes:   []string{"双色注塑", "纳米注塑", "包胶", "模内装饰"}, Regions: []string{"东莞市", "深圳市", "宁波市"}, Pinyin: "injection"},
	{Code: "diecasting", Name: "压铸", Short: "压铸", Category: "结构件",
		Description: "铝合金/镁合金高压压铸与半固态成形，适合中大型结构件",
		Processes:   []string{"高压压铸", "半固态成形", "CNC 后加工", "气密检测"}, Regions: []string{"宁波市", "东莞市", "重庆市"}, Pinyin: "diecasting"},
	{Code: "stamping", Name: "五金冲压", Short: "冲压", Category: "结构件",
		Description: "级进模冲压、深拉伸、精密弹片与屏蔽罩",
		Processes:   []string{"级进模", "深拉伸", "精密弹片", "连续冲压"}, Regions: []string{"东莞市", "昆山市", "宁波市"}, Pinyin: "stamping"},
	{Code: "mim", Name: "MIM 金属注射成形", Short: "金属成形", Category: "结构件",
		Description: "小型异形金属件的高精度批量成形，替代部分 CNC 工序",
		Processes:   []string{"喂料配制", "脱脂烧结", "精整", "抛光"}, Regions: []string{"深圳市", "苏州市", "东莞市"}, Pinyin: "mim"},
	{Code: "anodizing", Name: "阳极氧化", Short: "表面处理", Category: "结构件",
		Description: "铝/钛阳极氧化、硬质氧化与着色，控制色差与耐磨",
		Processes:   []string{"硬质氧化", "染色", "封孔", "色差管控"}, Regions: []string{"东莞市", "深圳市", "苏州市"}, Pinyin: "anodizing"},
	{Code: "coating", Name: "表面处理与镀膜", Short: "镀膜", Category: "结构件",
		Description: "UV 喷涂、PVD 真空镀、IMD/IML 与手感漆",
		Processes:   []string{"PVD", "UV 喷涂", "IMD", "手感漆"}, Regions: []string{"东莞市", "深圳市", "苏州市"}, Pinyin: "coating"},
	{Code: "glass", Name: "玻璃盖板", Short: "盖板", Category: "结构件",
		Description: "2.5D/3D 热弯、化学强化与 AF/AG 镀膜",
		Processes:   []string{"热弯", "化学强化", "AF 镀膜", "异形切割"}, Regions: []string{"深圳市", "惠州市", "昆山市"}, Pinyin: "glass"},
	{Code: "silicone", Name: "硅胶与橡胶成型", Short: "硅胶", Category: "结构件",
		Description: "液态硅胶、氟橡胶密封件与包胶件",
		Processes:   []string{"液态硅胶", "压缩成型", "氟橡胶", "二次硫化"}, Regions: []string{"东莞市", "深圳市", "厦门市"}, Pinyin: "silicone"},
	{Code: "band", Name: "表带与织物成型", Short: "表带", Category: "结构件",
		Description: "皮革、氟橡胶表带，编织带与声学网布",
		Processes:   []string{"皮革裁切", "车缝", "编织", "网布热压"}, Regions: []string{"东莞市", "广州市", "温州市"}, Pinyin: "band"},
	{Code: "fastener", Name: "精密紧固件", Short: "紧固件", Category: "结构件",
		Description: "螺丝、弹簧、精密车件与锁具五金",
		Processes:   []string{"冷镦", "搓丝", "热处理", "盐雾测试"}, Regions: []string{"东莞市", "宁波市", "温州市"}, Pinyin: "fastener"},
	{Code: "laser", Name: "激光加工", Short: "激光", Category: "结构件",
		Description: "精密切割、激光焊接与打标",
		Processes:   []string{"光纤切割", "激光焊接", "打标", "管材切割"}, Regions: []string{"东莞市", "深圳市", "苏州市"}, Pinyin: "laser"},
	{Code: "welding", Name: "焊接与钣金结构", Short: "焊接", Category: "结构件",
		Description: "机器人焊接、氩弧焊、钣金折弯与框架结构",
		Processes:   []string{"机器人焊接", "氩弧焊", "钣金折弯", "喷涂前处理"}, Regions: []string{"无锡市", "常州市", "天津市"}, Pinyin: "welding"},

	// ---------- 电子 ----------
	{Code: "pcb", Name: "PCB 制造", Short: "电路板", Category: "电子",
		Description: "HDI、多层板、软硬结合板，支持阻抗控制与小批量快板",
		Processes:   []string{"HDI 压合", "阻抗控制", "沉金", "飞针测试"}, Regions: []string{"深圳市", "苏州市", "惠州市"}, Pinyin: "pcb"},
	{Code: "pcba", Name: "SMT 与 PCBA", Short: "电子制造", Category: "电子",
		Description: "01005 贴装、AOI/X-Ray 检测、ICT/FCT 功能测试",
		Processes:   []string{"锡膏印刷", "高速贴片", "回流焊", "X-Ray 检测", "功能测试"}, Regions: []string{"深圳市", "东莞市", "苏州市"}, Pinyin: "pcba"},
	{Code: "fpc", Name: "FPC 柔性电路", Short: "柔性电路", Category: "电子",
		Description: "细线路多层软板、软硬结合与补强",
		Processes:   []string{"细线路蚀刻", "覆盖膜压合", "补强", "弯折测试"}, Regions: []string{"深圳市", "珠海市"}, Pinyin: "fpc"},
	{Code: "display", Name: "显示模组", Short: "显示模组", Category: "电子",
		Description: "AMOLED/LCD 模组贴合、全贴合与驱动调试",
		Processes:   []string{"COG/COF 绑定", "全贴合", "光学检测", "Gamma 校准"}, Regions: []string{"深圳市", "惠州市", "昆山市"}, Pinyin: "display"},
	{Code: "camera", Name: "摄像头模组", Short: "摄像头", Category: "电子",
		Description: "CMOS 封装、AA 主动对位组装与影像调校",
		Processes:   []string{"AA 组装", "镜头座贴装", "影像调校", "脏污管控"}, Regions: []string{"深圳市", "宁波市", "南昌市"}, Pinyin: "camera"},
	{Code: "sensor", Name: "传感器模组", Short: "传感器", Category: "电子",
		Description: "PPG 光学心率、IMU、压力与激光雷达模组",
		Processes:   []string{"光学标定", "贴装应力控制", "老化筛选", "算法联调"}, Regions: []string{"深圳市", "苏州市", "上海市"}, Pinyin: "sensor"},
	{Code: "battery", Name: "锂电芯与 PACK", Short: "电池", Category: "电源",
		Description: "聚合物电芯、异形电芯与保护板 PACK",
		Processes:   []string{"叠片", "注液化成", "分容分选", "PACK 组装", "UN38.3 认证"}, Regions: []string{"深圳市", "东莞市", "惠州市"}, Pinyin: "battery"},
	{Code: "charger", Name: "电源与无线充电", Short: "电源", Category: "电源",
		Description: "快充协议、无线充电模组与适配器",
		Processes:   []string{"Qi 线圈绕制", "协议调试", "温升测试", "效率测试"}, Regions: []string{"深圳市", "东莞市", "苏州市"}, Pinyin: "charger"},
	{Code: "motor", Name: "微电机与驱动", Short: "电机", Category: "电源",
		Description: "无刷电机、线性马达、减速箱与驱动控制",
		Processes:   []string{"绕线", "磁钢装配", "动平衡", "寿命测试"}, Regions: []string{"深圳市", "东莞市", "常州市"}, Pinyin: "motor"},
	{Code: "acoustics", Name: "声学器件", Short: "声学", Category: "电子",
		Description: "扬声器、受话器、MEMS 麦克风与腔体调音",
		Processes:   []string{"振膜成型", "腔体调音", "频响测试", "防水处理"}, Regions: []string{"深圳市", "苏州市", "厦门市"}, Pinyin: "acoustics"},
	{Code: "connector", Name: "连接器与线束", Short: "连接器", Category: "电子",
		Description: "pogo pin、Type-C、FPC 连接器与精密线束",
		Processes:   []string{"冲压端子", "电镀", "注塑成型", "插拔寿命测试"}, Regions: []string{"深圳市", "东莞市", "苏州市"}, Pinyin: "connector"},
	{Code: "magnet", Name: "磁性材料", Short: "磁材", Category: "电子",
		Description: "钕铁硼、磁组件与充磁方案",
		Processes:   []string{"烧结", "精加工", "充磁", "磁通检测"}, Regions: []string{"宁波市", "深圳市", "赣州市"}, Pinyin: "magnet"},
	{Code: "lens", Name: "光学镜头", Short: "光学", Category: "电子",
		Description: "塑胶/玻璃镜片、镀膜与镜头组装",
		Processes:   []string{"镜片注塑", "研磨抛光", "镀膜", "MTF 检测"}, Regions: []string{"深圳市", "宁波市", "厦门市"}, Pinyin: "lens"},
	{Code: "antenna", Name: "天线与射频", Short: "天线", Category: "电子",
		Description: "LDS 天线、FPC 天线与射频调试认证",
		Processes:   []string{"LDS 化镀", "天线调试", "SAR 测试", "OTA 测试"}, Regions: []string{"深圳市", "苏州市"}, Pinyin: "antenna"},
	{Code: "heatsink", Name: "散热与导热", Short: "散热", Category: "结构件",
		Description: "热管、VC 均温板、导热硅胶垫与散热模组",
		Processes:   []string{"热管封装", "VC 焊接", "导热测试", "模切"}, Regions: []string{"深圳市", "东莞市", "苏州市"}, Pinyin: "heatsink"},

	// ---------- 组装服务 ----------
	{Code: "assembly", Name: "整机组装", Short: "整机", Category: "组装服务",
		Description: "洁净车间、自动化装配线与成品包装交付",
		Processes:   []string{"自动化装配", "点胶压合", "在线老化", "成品抽检"}, Regions: []string{"东莞市", "深圳市", "苏州市"}, Pinyin: "assembly"},
	{Code: "testing", Name: "可靠性与认证", Short: "检测", Category: "组装服务",
		Description: "IP68 防护、跌落、温循、EMC 与第三方认证",
		Processes:   []string{"气密测试", "跌落测试", "高低温循环", "EMC 预测试"}, Regions: []string{"深圳市", "东莞市", "苏州市"}, Pinyin: "testing"},
	{Code: "packaging", Name: "包装印刷", Short: "包装", Category: "组装服务",
		Description: "彩盒、纸浆内托、说明书与环保材料",
		Processes:   []string{"胶印", "模切", "纸浆成型", "手工组装"}, Regions: []string{"东莞市", "深圳市", "温州市"}, Pinyin: "packaging"},
}

var capabilityIndex = func() map[string]Capability {
	index := make(map[string]Capability, len(Capabilities))
	for _, c := range Capabilities {
		index[c.Code] = c
	}
	return index
}()

func CapabilityByCode(code string) (Capability, bool) {
	c, ok := capabilityIndex[code]
	return c, ok
}

// CapabilityCodes 返回全部能力代码，用于初始化与校验。
func CapabilityCodes() []string {
	codes := make([]string, 0, len(Capabilities))
	for _, c := range Capabilities {
		codes = append(codes, c.Code)
	}
	return codes
}
