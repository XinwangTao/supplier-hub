package catalog

import "strings"

// Process 是产品配方里的一个工序节点，引用一类工艺能力。
// 不同的产品可以复用同一类能力（例如手表和无人机都要用 CNC），
// 这正是供应链网络能够交叉复用的原因。
type Process struct {
	Capability string `json:"capability"`
	Name       string `json:"name"`
	Spec       string `json:"spec"`
	Critical   bool   `json:"critical"` // 关键工序：缺供或单点会直接卡住量产
	Qty        string `json:"qty"`      // 典型单机用量，帮助理解 BOM 结构
}

type Module struct {
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	Desc      string    `json:"desc"`
	Processes []Process `json:"processes"`
}

type Recipe struct {
	Product  string   `json:"product"`
	Aliases  []string `json:"aliases"`
	Category string   `json:"category"`
	Summary  string   `json:"summary"`
	Modules  []Module `json:"modules"`
}

var Recipes = []Recipe{
	{
		Product:  "智能手表",
		Aliases:  []string{"手表", "腕表", "运动手表", "smartwatch", "watch", "智能穿戴"},
		Category: "可穿戴",
		Summary:  "以主板与显示为核心，结构件与表带决定外观，防水与传感决定体验，整机组装是量产卡点。",
		Modules: []Module{
			{Key: "core", Name: "主板与计算", Desc: "低功耗 SoC、蓝牙与 GNSS 射频，是整机成本与功耗的核心。", Processes: []Process{
				{Capability: "pcba", Name: "主控 PCBA", Spec: "BLE 5.3 + GNSS SoC，01005 元件贴装，需 X-Ray 与 ICT 全检", Critical: true, Qty: "1 片"},
				{Capability: "pcb", Name: "HDI 主板", Spec: "6 层二阶级 HDI，阻抗 ±10%，板厚 0.6mm", Critical: true, Qty: "1 片"},
				{Capability: "antenna", Name: "天线与射频调试", Spec: "LDS 天线，BLE/GNSS 双频，SAR 与 OTA 摸底", Qty: "2 副"},
			}},
			{Key: "display", Name: "显示与触控", Desc: "圆形 AMOLED 与曲面盖板，贴合良率决定整机观感。", Processes: []Process{
				{Capability: "display", Name: "AMOLED 显示模组", Spec: "1.43 英寸圆形，PPI≥326，全贴合 + 圆角异形切割", Critical: true, Qty: "1 片"},
				{Capability: "glass", Name: "2.5D 曲面盖板", Spec: "2.5D 弧边，AG/AF 双镀，落球 1.2m 不破", Qty: "1 片"},
			}},
			{Key: "power", Name: "能源与充电", Desc: "异形电芯塞进弧形表壳，无线充与密封是难点。", Processes: []Process{
				{Capability: "battery", Name: "异形聚合物电芯", Spec: "350–500mAh 弧形电芯，厚度 ≤4.2mm，循环 500 次", Critical: true, Qty: "1 颗"},
				{Capability: "charger", Name: "无线充电模组", Spec: "5W 磁吸 Qi，温升 ≤8℃，线圈厚度 ≤0.6mm", Qty: "1 套"},
				{Capability: "connector", Name: "磁吸充电触点", Spec: "pogo pin 镀金 3μm，IP68 密封结构", Qty: "2 组"},
			}},
			{Key: "sensor", Name: "传感与交互", Desc: "心率血氧、IMU、马达与声学，决定健康与交互能力。", Processes: []Process{
				{Capability: "sensor", Name: "PPG 心率血氧模组", Spec: "双绿光 + 红外，运动伪影抑制，贴装应力管控", Critical: true, Qty: "1 组"},
				{Capability: "sensor", Name: "六轴 IMU 与气压计", Spec: "加速度+陀螺仪，偏移 ≤0.5°，回流后校准", Qty: "1 组"},
				{Capability: "motor", Name: "X 轴线性马达", Spec: "振感量 1.6Grms，厚度 ≤3.2mm", Qty: "1 颗"},
				{Capability: "acoustics", Name: "防水扬声器与麦克风", Spec: "防水膜扬声器 + MEMS 麦，腔体密封 IP68", Qty: "1 套"},
			}},
			{Key: "structure", Name: "结构与外观", Desc: "中框、后盖、表冠与密封圈，直接决定质感与防水。", Processes: []Process{
				{Capability: "cnc", Name: "一体中框 CNC", Spec: "6061 铝 / 钛合金，公差 ±0.02mm，4 轴联动", Critical: true, Qty: "1 件"},
				{Capability: "anodizing", Name: "阳极氧化着色", Spec: "硬质氧化 ≥15μm，色差 ΔE<1，耐磨 500g 荷重", Qty: "1 件"},
				{Capability: "injection", Name: "后盖双色注塑", Spec: "纳米注塑 + 天线断点，缩水率 ≤0.3%", Qty: "1 件"},
				{Capability: "silicone", Name: "防水密封圈", Spec: "氟橡胶 O 型圈，压缩永久变形 ≤20%", Qty: "2 件"},
				{Capability: "mim", Name: "表冠与侧键", Spec: "MIM 不锈钢表冠，抛光 Ra0.2，盐雾 48h", Qty: "3 件"},
			}},
			{Key: "band", Name: "表带", Desc: "快拆结构 + 材质工艺，是差异化卖点也是售后高发件。", Processes: []Process{
				{Capability: "band", Name: "氟橡胶/液态硅胶表带", Spec: "耐汗液老化 96h，拉伸强度 ≥8MPa", Qty: "1 条"},
				{Capability: "stamping", Name: "金属表带链节", Spec: "不锈钢链节冲压 + 抛光，插拔寿命 5000 次", Qty: "1 条"},
			}},
			{Key: "assembly", Name: "组装与测试", Desc: "洁净车间装配，防水与老化测试是出货闸门。", Processes: []Process{
				{Capability: "assembly", Name: "整机组装", Spec: "万级洁净车间，自动点胶压合，直通率 ≥98%", Critical: true, Qty: "1 台"},
				{Capability: "testing", Name: "IP68 与可靠性验证", Spec: "1ATM 气密 + 50m 水下老化，1.5m 六面跌落，温循 -20~60℃", Critical: true, Qty: "抽检"},
			}},
			{Key: "pack", Name: "包装与配件", Desc: "环保包装与充电底座配套。", Processes: []Process{
				{Capability: "packaging", Name: "彩盒与纸浆内托", Spec: "FSC 认证纸张，纸浆内托，跌落 1.2m 通过", Qty: "1 套"},
				{Capability: "charger", Name: "磁吸充电底座", Spec: "同色阳极外壳，5W 输出，Type-C 输入", Qty: "1 个"},
			}},
		},
	},
	{
		Product:  "TWS 蓝牙耳机",
		Aliases:  []string{"tws", "蓝牙耳机", "耳机", "earbuds", "无线耳机", "降噪耳机"},
		Category: "可穿戴",
		Summary:  "声学腔体与主板堆叠是核心，微型电芯与充电仓决定续航，组装精度影响一致性。",
		Modules: []Module{
			{Key: "acoustic", Name: "声学系统", Desc: "喇叭、麦克风与腔体调音决定听感与通话降噪。", Processes: []Process{
				{Capability: "acoustics", Name: "13mm 动圈扬声器", Spec: "频响 20Hz–20kHz，THD ≤1%，防水膜", Critical: true, Qty: "2 颗"},
				{Capability: "acoustics", Name: "MEMS 麦克风阵列", Spec: "双麦 ENC 降噪，SNR ≥65dB", Qty: "6 颗"},
				{Capability: "silicone", Name: "液态硅胶耳塞", Spec: "邵氏 30A，生物相容性认证", Qty: "6 只"},
			}},
			{Key: "board", Name: "主板与射频", Desc: "耳机柄内软硬结合板，空间极度受限。", Processes: []Process{
				{Capability: "pcba", Name: "蓝牙主控 PCBA", Spec: "BLE 5.3 SoC，0201 贴装，板宽 ≤4mm", Critical: true, Qty: "2 片"},
				{Capability: "fpc", Name: "软硬结合板", Spec: "4 层软硬结合，弯折 10 万次", Qty: "2 片"},
				{Capability: "antenna", Name: "蓝牙天线", Spec: "FPC 天线，OTA 效率 ≥60%", Qty: "2 副"},
			}},
			{Key: "power", Name: "电源与充电", Desc: "针型电芯 + 充电仓，续航与快充体验的关键。", Processes: []Process{
				{Capability: "battery", Name: "针型/扣式电芯", Spec: "45–60mAh，循环 500 次，针刺不起火", Critical: true, Qty: "2 颗"},
				{Capability: "connector", Name: "pogo pin 充电触点", Spec: "镀金 3μm，插拔 1 万次，IPX4", Qty: "6 组"},
				{Capability: "charger", Name: "充电仓电源板", Spec: "Type-C 快充 + 无线充，500mAh 仓电", Qty: "1 套"},
			}},
			{Key: "structure", Name: "结构件", Desc: "耳机壳体精度决定装配间隙与声学密封。", Processes: []Process{
				{Capability: "injection", Name: "耳机壳体注塑", Spec: "双色注塑，公差 ±0.03mm，IPX4 密封", Critical: true, Qty: "2 套"},
				{Capability: "mold", Name: "精密模具", Spec: "一模多穴，镜面抛光，寿命 50 万模次", Qty: "1 套"},
				{Capability: "magnet", Name: "霍尔磁铁", Spec: "钕铁硼 N45，磁通一致性 ±3%", Qty: "4 颗"},
			}},
			{Key: "assembly", Name: "组装与测试", Desc: "声学测试与配对测试是出货闸门。", Processes: []Process{
				{Capability: "assembly", Name: "整机组装", Spec: "自动化点胶 + 超声焊接，直通率 ≥97%", Critical: true, Qty: "1 套"},
				{Capability: "testing", Name: "声学与可靠性测试", Spec: "频响曲线全检，1.5m 跌落，汗液腐蚀 48h", Qty: "抽检"},
			}},
			{Key: "pack", Name: "包装", Desc: "小体积高质感包装。", Processes: []Process{
				{Capability: "packaging", Name: "彩盒与内托", Spec: "环保纸浆内托 + 说明卡", Qty: "1 套"},
			}},
		},
	},
	{
		Product:  "扫地机器人",
		Aliases:  []string{"扫地机", "扫拖机器人", "robot vacuum", "扫地机器人", "洗地机"},
		Category: "智能家电",
		Summary:  "激光雷达与主控决定导航能力，电机与电池决定清洁力与续航，结构件数量多、模具投入大。",
		Modules: []Module{
			{Key: "nav", Name: "导航与感知", Desc: "激光雷达 + IMU + 沿墙传感，是算法落地的硬件基础。", Processes: []Process{
				{Capability: "sensor", Name: "LDS 激光雷达", Spec: "360° 扫描，测距 8m，寿命 1 万小时", Critical: true, Qty: "1 颗"},
				{Capability: "sensor", Name: "沿墙与防跌落传感", Spec: "红外对管阵列，响应 ≤10ms", Qty: "8 组"},
				{Capability: "pcba", Name: "主控与算法板", Spec: "AI 视觉 SoC，6 层板，EMC 摸底", Critical: true, Qty: "1 片"},
			}},
			{Key: "power", Name: "动力与续航", Desc: "行走电机、风机与电池包构成动力系统。", Processes: []Process{
				{Capability: "motor", Name: "无刷行走电机", Spec: "轮毂减速箱，噪音 ≤55dB，寿命 1500h", Critical: true, Qty: "2 颗"},
				{Capability: "motor", Name: "无刷风机马达", Spec: "吸力 ≥5000Pa，转速 3 万转", Critical: true, Qty: "1 颗"},
				{Capability: "battery", Name: "锂电池包", Spec: "5200mAh，BMS 保护，UN38.3", Critical: true, Qty: "1 组"},
				{Capability: "charger", Name: "自动回充座", Spec: "红外对接，20V/2A 快充", Qty: "1 套"},
			}},
			{Key: "clean", Name: "清洁结构", Desc: "尘盒、水箱、滚刷与密封，直接影响清洁效果。", Processes: []Process{
				{Capability: "injection", Name: "尘盒与水箱注塑", Spec: "透明 PC + 双色包胶，气密 30kPa", Critical: true, Qty: "2 套"},
				{Capability: "mold", Name: "大型结构模具", Spec: "一模多穴，模温控制，寿命 30 万模次", Qty: "6 套"},
				{Capability: "silicone", Name: "密封条与滚刷胶条", Spec: "邵氏 50A，耐磨 5 万次", Qty: "4 件"},
				{Capability: "stamping", Name: "金属结构件", Spec: "1.5mm 冷轧板冲压 + 喷粉", Qty: "3 件"},
			}},
			{Key: "assembly", Name: "组装与测试", Desc: "整机噪音、寿命与跌落是主要验证项。", Processes: []Process{
				{Capability: "assembly", Name: "整机组装", Spec: "自动化装配线，节拍 ≤45s/台", Critical: true, Qty: "1 台"},
				{Capability: "testing", Name: "整机寿命与噪音", Spec: "1000h 连续运行，噪音 ≤65dB，1m 跌落", Qty: "抽检"},
			}},
			{Key: "pack", Name: "包装", Desc: "大件包装与缓冲设计。", Processes: []Process{
				{Capability: "packaging", Name: "重型彩箱与缓冲", Spec: "五层瓦楞，ISTA 3A 通过", Qty: "1 套"},
			}},
		},
	},
	{
		Product:  "移动电源",
		Aliases:  []string{"充电宝", "power bank", "移动电源", "快充宝"},
		Category: "消费电子配件",
		Summary:  "电芯与 BMS 是安全底线，外壳与表面处理决定溢价，组装与老化测试决定返修率。",
		Modules: []Module{
			{Key: "cell", Name: "电芯与 BMS", Desc: "电芯一致性 + 保护板方案，是安全与容量的核心。", Processes: []Process{
				{Capability: "battery", Name: "21700/聚合物电芯", Spec: "5000mAh，内阻一致性 ±5%，UN38.3", Critical: true, Qty: "2–4 颗"},
				{Capability: "pcba", Name: "BMS 保护与快充板", Spec: "PD 100W 双向快充，过温过流保护", Critical: true, Qty: "1 片"},
				{Capability: "pcb", Name: "4 层电源板", Spec: "2oz 铜厚，温升 ≤40K", Qty: "1 片"},
				{Capability: "heatsink", Name: "导热与散热垫", Spec: "导热系数 3W/mK，模切成型", Qty: "2 片"},
			}},
			{Key: "shell", Name: "外壳与外观", Desc: "阻燃外壳 + 表面处理，兼顾安全与手感。", Processes: []Process{
				{Capability: "injection", Name: "阻燃外壳注塑", Spec: "PC+ABS V0 阻燃，双色包胶", Critical: true, Qty: "1 套"},
				{Capability: "coating", Name: "手感漆与 PVD", Spec: "橡胶漆，耐磨 500g 荷重 500 次", Qty: "1 套"},
				{Capability: "mold", Name: "外壳模具", Spec: "高光/蚀纹，寿命 30 万模次", Qty: "2 套"},
			}},
			{Key: "interface", Name: "接口与显示", Desc: "接口寿命与电量显示是体验细节。", Processes: []Process{
				{Capability: "connector", Name: "Type-C 母座", Spec: "16pin 双面插，插拔 1 万次", Critical: true, Qty: "1–2 颗"},
				{Capability: "display", Name: "数码管电量显示", Spec: "LED 数码管，低功耗驱动", Qty: "1 套"},
			}},
			{Key: "assembly", Name: "组装与测试", Desc: "老化与短路测试必须全检。", Processes: []Process{
				{Capability: "assembly", Name: "整机组装", Spec: "超声波焊接 + 自动化测试", Critical: true, Qty: "1 台"},
				{Capability: "testing", Name: "老化与安全测试", Spec: "4h 满载老化，短路/过充/跌落全检", Critical: true, Qty: "全检"},
			}},
			{Key: "pack", Name: "包装", Desc: "含认证标识与说明书。", Processes: []Process{
				{Capability: "packaging", Name: "彩盒与说明", Spec: "含 3C/CE 标识，环保油墨", Qty: "1 套"},
			}},
		},
	},
	{
		Product:  "无人机",
		Aliases:  []string{"无人机", "drone", "航拍机", "飞行器"},
		Category: "智能硬件",
		Summary:  "飞控与图传是技术门槛，动力系统决定载重与续航，云台相机决定成像，结构件以碳纤与 CNC 为主。",
		Modules: []Module{
			{Key: "flight", Name: "飞控与图传", Desc: "飞控稳定性与图传距离是产品分级的关键。", Processes: []Process{
				{Capability: "pcba", Name: "飞控主板", Spec: "双 IMU 冗余，6 层板，-20~60℃ 工作", Critical: true, Qty: "1 片"},
				{Capability: "sensor", Name: "IMU/气压计/罗盘", Spec: "零偏稳定性 ≤10°/h，出厂标定", Critical: true, Qty: "2 组"},
				{Capability: "antenna", Name: "图传天线", Spec: "2.4/5.8G 双频，图传 ≥10km", Qty: "4 副"},
			}},
			{Key: "power", Name: "动力系统", Desc: "电机、桨叶与智能电池决定载重与续航。", Processes: []Process{
				{Capability: "motor", Name: "无刷电机", Spec: "KV 值一致性 ±2%，动平衡 ≤0.5g·mm", Critical: true, Qty: "4 颗"},
				{Capability: "battery", Name: "智能电池包", Spec: "4S 5000mAh，自放电保护，低温预热", Critical: true, Qty: "1 组"},
				{Capability: "charger", Name: "快充适配器", Spec: "100W 双路输出，温控降额", Qty: "1 个"},
			}},
			{Key: "camera", Name: "云台与相机", Desc: "三轴云台 + 4K 相机，机械与光学精度要求高。", Processes: []Process{
				{Capability: "camera", Name: "4K 相机模组", Spec: "1/1.3 英寸 CMOS，4K60fps，AA 组装", Critical: true, Qty: "1 颗"},
				{Capability: "lens", Name: "广角镜头", Spec: "24mm 等效，畸变 ≤2%，多层镀膜", Critical: true, Qty: "1 组"},
				{Capability: "glass", Name: "镜头保护玻璃", Spec: "超白玻，AR 镀膜，透光率 ≥98%", Qty: "1 片"},
				{Capability: "mim", Name: "云台精密结构件", Spec: "MIM 钛合金件，重量 ≤2g/件", Qty: "6 件"},
			}},
			{Key: "frame", Name: "机身与机臂", Desc: "轻量化与刚性之间的平衡。", Processes: []Process{
				{Capability: "cnc", Name: "机臂与机身 CNC", Spec: "7075 铝 / 碳纤板，公差 ±0.05mm", Critical: true, Qty: "1 套"},
				{Capability: "mold", Name: "桨叶模具", Spec: "高精度动平衡模具，寿命 20 万模次", Qty: "2 套"},
				{Capability: "injection", Name: "机身外壳注塑", Spec: "PC+玻纤，耐候 UV 12 个月", Qty: "1 套"},
			}},
			{Key: "assembly", Name: "组装与测试", Desc: "整机校准与振动测试决定飞行安全。", Processes: []Process{
				{Capability: "assembly", Name: "整机组装与校准", Spec: "动平衡、罗盘校准、桨叶配平", Critical: true, Qty: "1 台"},
				{Capability: "testing", Name: "振动与跌落测试", Spec: "振动台 10h，1m 跌落，EMC 摸底", Critical: true, Qty: "抽检"},
			}},
			{Key: "pack", Name: "包装", Desc: "防震包装与配件收纳。", Processes: []Process{
				{Capability: "packaging", Name: "防震箱与内衬", Spec: "EPP 内衬，ISTA 3A 通过", Qty: "1 套"},
			}},
		},
	},
	{
		Product:  "电动自行车",
		Aliases:  []string{"ebike", "电助力车", "电动车", "电动单车", "电动自行车"},
		Category: "出行工具",
		Summary:  "车架焊接与涂装是产能瓶颈，电机、电池与控制器构成三电核心，认证周期长。",
		Modules: []Module{
			{Key: "frame", Name: "车架与结构", Desc: "管材切割、焊接与涂装，决定刚性与外观。", Processes: []Process{
				{Capability: "laser", Name: "管材激光切割", Spec: "切缝 ≤0.2mm，批量一致性 ±0.1mm", Qty: "12 件"},
				{Capability: "welding", Name: "车架机器人焊接", Spec: "氩弧焊 + 机器人焊接，焊缝探伤", Critical: true, Qty: "1 套"},
				{Capability: "stamping", Name: "冲压连接件", Spec: "3mm 高强钢冲压，盐雾 96h", Qty: "10 件"},
				{Capability: "coating", Name: "静电喷涂", Spec: "三道涂层，附着力 1 级，耐候 3 年", Qty: "1 套"},
			}},
			{Key: "drive", Name: "三电系统", Desc: "电机、电池与控制器，成本占比最高。", Processes: []Process{
				{Capability: "motor", Name: "轮毂电机", Spec: "250W/350W，噪音 ≤55dB，防水 IPX5", Critical: true, Qty: "1 颗"},
				{Capability: "battery", Name: "电池包", Spec: "36V 10–15Ah，BMS + 锁芯结构", Critical: true, Qty: "1 组"},
				{Capability: "pcba", Name: "控制器与仪表板", Spec: "FOC 矢量控制，防水灌胶", Critical: true, Qty: "1 套"},
				{Capability: "display", Name: "仪表显示屏", Spec: "2.4 英寸 IPS，阳光下可读", Qty: "1 块"},
			}},
			{Key: "chassis", Name: "传动与制动", Desc: "制动安全件必须满足认证要求。", Processes: []Process{
				{Capability: "fastener", Name: "高强度紧固件", Spec: "10.9 级，盐雾 240h", Qty: "40 颗"},
				{Capability: "cnc", Name: "刹车与传动件", Spec: "碟刹盘 CNC，平面度 ≤0.05mm", Qty: "4 件"},
			}},
			{Key: "assembly", Name: "组装与测试", Desc: "整车路试与认证。", Processes: []Process{
				{Capability: "assembly", Name: "整车组装", Spec: "流水线节拍 ≤120s/台", Critical: true, Qty: "1 台"},
				{Capability: "testing", Name: "整车路试与认证", Spec: "50km 路试，EN15194/CE 认证", Critical: true, Qty: "抽检"},
			}},
			{Key: "pack", Name: "包装", Desc: "大件出口包装。", Processes: []Process{
				{Capability: "packaging", Name: "出口纸箱与木架", Spec: "七层瓦楞 + 木架，海运防潮", Qty: "1 套"},
			}},
		},
	},
	{
		Product:  "智能门锁",
		Aliases:  []string{"门锁", "指纹锁", "智能锁", "智能门锁"},
		Category: "智能家居",
		Summary:  "面板五金质感与锁体机械可靠性并重，指纹识别与猫眼构成智能部分，安装适配是隐性门槛。",
		Modules: []Module{
			{Key: "panel", Name: "面板与外观", Desc: "金属面板的质感与工艺直接决定售价带。", Processes: []Process{
				{Capability: "stamping", Name: "面板冲压", Spec: "2mm 不锈钢/锌合金，拉丝纹路", Critical: true, Qty: "2 件"},
				{Capability: "cnc", Name: "面板 CNC 精修", Spec: "高光倒角，Ra0.4，公差 ±0.03mm", Qty: "2 件"},
				{Capability: "anodizing", Name: "阳极氧化/镀层", Spec: "哑黑/香槟金，耐磨 500g 荷重", Qty: "2 件"},
				{Capability: "coating", Name: "防指纹涂层", Spec: "AF 涂层，接触角 ≥110°", Qty: "2 件"},
			}},
			{Key: "lock", Name: "锁体与机械", Desc: "机械寿命与防技术开启是安全底线。", Processes: []Process{
				{Capability: "diecasting", Name: "压铸锁体", Spec: "锌合金压铸，10 万次开合寿命", Critical: true, Qty: "1 套"},
				{Capability: "fastener", Name: "锁芯与紧固件", Spec: "C 级锁芯，防钻 30min", Critical: true, Qty: "1 套"},
				{Capability: "mim", Name: "精密传动件", Spec: "MIM 齿轮与拨片，耐磨处理", Qty: "8 件"},
			}},
			{Key: "identify", Name: "识别与视觉", Desc: "指纹、人脸与猫眼，是智能化的核心体验。", Processes: []Process{
				{Capability: "sensor", Name: "半导体指纹模组", Spec: "识别 ≤0.5s，FAR ≤0.001%", Critical: true, Qty: "1 颗"},
				{Capability: "camera", Name: "猫眼摄像头", Spec: "2MP 广角 180°，红外夜视", Qty: "1 颗"},
				{Capability: "display", Name: "室内显示屏", Spec: "3.5 英寸 IPS，低功耗常亮", Qty: "1 块"},
			}},
			{Key: "control", Name: "主控与供电", Desc: "低功耗主控 + 双电源冗余，断电可用是刚需。", Processes: []Process{
				{Capability: "pcba", Name: "主控与无线板", Spec: "WiFi/BLE 双模，低功耗待机 12 个月", Critical: true, Qty: "1 片"},
				{Capability: "battery", Name: "电池仓与备用电源", Spec: "8 节 AA 或锂电包，Type-C 应急供电", Qty: "1 组"},
				{Capability: "connector", Name: "应急供电触点", Spec: "Type-C 防水母座，IP65", Qty: "1 颗"},
			}},
			{Key: "assembly", Name: "组装与测试", Desc: "机械寿命与电子功能全检。", Processes: []Process{
				{Capability: "assembly", Name: "整机组装", Spec: "自动化锁体装配 + 功能测试", Critical: true, Qty: "1 套"},
				{Capability: "testing", Name: "寿命与安全测试", Spec: "10 万次开合，防撬报警，EMC", Critical: true, Qty: "抽检"},
			}},
			{Key: "pack", Name: "包装", Desc: "含安装配件与开孔图。", Processes: []Process{
				{Capability: "packaging", Name: "彩盒与安装配件包", Spec: "含开孔模板与膨胀螺栓", Qty: "1 套"},
			}},
		},
	},
	{
		Product:  "蓝牙音箱",
		Aliases:  []string{"音箱", "音响", "speaker", "蓝牙音箱", "便携音箱"},
		Category: "消费电子",
		Summary:  "声学腔体与外观织物是卖点，电池与快充决定便携性，防水等级决定使用场景。",
		Modules: []Module{
			{Key: "acoustic", Name: "声学系统", Desc: "全频单元 + 被动辐射器，低频下潜靠腔体设计。", Processes: []Process{
				{Capability: "acoustics", Name: "全频扬声器单元", Spec: "45mm 全频，频响 60Hz–20kHz", Critical: true, Qty: "2 颗"},
				{Capability: "acoustics", Name: "被动辐射器", Spec: "低频下潜至 55Hz，防水膜", Qty: "2 颗"},
				{Capability: "silicone", Name: "腔体密封与脚垫", Spec: "硅胶密封圈，气密性 ≥10kPa", Qty: "4 件"},
			}},
			{Key: "board", Name: "主板与射频", Desc: "功放与蓝牙方案决定音质上限。", Processes: []Process{
				{Capability: "pcba", Name: "功放与蓝牙主板", Spec: "Class-D 功放 2×20W，BLE 5.3", Critical: true, Qty: "1 片"},
				{Capability: "pcb", Name: "4 层音频板", Spec: "模拟数字分区，底噪 ≤20μV", Qty: "1 片"},
				{Capability: "antenna", Name: "蓝牙天线", Spec: "PCB 天线，OTA 效率 ≥55%", Qty: "1 副"},
			}},
			{Key: "power", Name: "电源", Desc: "续航与快充是便携场景刚需。", Processes: []Process{
				{Capability: "battery", Name: "锂电 pack", Spec: "5000mAh，续航 ≥12h", Critical: true, Qty: "1 组"},
				{Capability: "charger", Name: "Type-C 快充与升压", Spec: "PD 20W 输入，升压 12V 功放供电", Qty: "1 套"},
				{Capability: "connector", Name: "Type-C 接口", Spec: "IPX7 防水母座", Qty: "1 颗"},
			}},
			{Key: "shell", Name: "外壳与网布", Desc: "织物网布 + 双色注塑，兼顾声学透声与防水。", Processes: []Process{
				{Capability: "injection", Name: "壳体注塑", Spec: "双色注塑，IPX7 结构", Critical: true, Qty: "1 套"},
				{Capability: "band", Name: "声学网布", Spec: "透声损耗 ≤1dB，耐磨 2 万次", Qty: "1 套"},
				{Capability: "mold", Name: "壳体模具", Spec: "高光 + 蚀纹，寿命 30 万模次", Qty: "2 套"},
			}},
			{Key: "assembly", Name: "组装与测试", Desc: "气密与频响全检。", Processes: []Process{
				{Capability: "assembly", Name: "整机组装", Spec: "超声焊接密封，自动化装配", Critical: true, Qty: "1 台"},
				{Capability: "testing", Name: "气密与频响测试", Spec: "IPX7 浸水 30min，频响全检", Critical: true, Qty: "全检"},
			}},
			{Key: "pack", Name: "包装", Desc: "挂卡或彩盒。", Processes: []Process{
				{Capability: "packaging", Name: "彩盒与挂卡", Spec: "环保油墨，跌落 1m 通过", Qty: "1 套"},
			}},
		},
	},
}

// MatchRecipe 根据用户输入（可能是「我想做一个智能手表」这类自然语言）匹配产品配方。
// 采用最长别名匹配，避免「耳机」误匹配到「蓝牙音箱」之类的短词冲突。
func MatchRecipe(query string) (Recipe, bool) {
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized == "" {
		return Recipe{}, false
	}
	best, bestLen := Recipe{}, 0
	for _, r := range Recipes {
		candidates := append([]string{r.Product}, r.Aliases...)
		for _, alias := range candidates {
			key := strings.ToLower(alias)
			if strings.Contains(normalized, key) && len([]rune(key)) > bestLen {
				best, bestLen = r, len([]rune(key))
			}
		}
	}
	return best, bestLen > 0
}

// RecipeByProduct 精确取配方。
func RecipeByProduct(product string) (Recipe, bool) {
	for _, r := range Recipes {
		if r.Product == product {
			return r, true
		}
	}
	return Recipe{}, false
}

// GenericRecipe 为未收录的产品做通用拆解：结构件 + 电子 + 电源 + 组装。
// 用户输入任何新产品时，网络图仍然能"长"出来，而不是报错。
func GenericRecipe(product string) Recipe {
	name := strings.TrimSpace(product)
	if name == "" {
		name = "新硬件产品"
	}
	return Recipe{
		Product:  name,
		Category: "通用硬件",
		Summary:  "该产品尚未收录专用配方，已按通用硬件结构拆解为结构件、电子、电源与组装测试四条主线，可在此基础上继续补充专用工序。",
		Modules: []Module{
			{Key: "generic-structure", Name: "结构与外观", Desc: "外壳、支架与外观件，通常从模具与注塑起步。", Processes: []Process{
				{Capability: "injection", Name: "外壳注塑", Spec: "按结构图纸开模，常规 PC/ABS 材质", Critical: true},
				{Capability: "mold", Name: "结构模具", Spec: "首模 + 修模，常规寿命 20–30 万模次"},
				{Capability: "cnc", Name: "精密结构件", Spec: "铝合金 CNC + 表面处理"},
			}},
			{Key: "generic-electronics", Name: "电子与主控", Desc: "主板、贴装与连接，几乎是一切带电产品的公共底座。", Processes: []Process{
				{Capability: "pcba", Name: "主控 PCBA", Spec: "方案选型 + SMT 贴装 + 功能测试", Critical: true},
				{Capability: "pcb", Name: "PCB 板", Spec: "2–6 层板，按信号完整性选型"},
				{Capability: "connector", Name: "接口与线束", Spec: "按接口定义选型，含插拔寿命要求"},
			}},
			{Key: "generic-power", Name: "电源与驱动", Desc: "电池、电源与电机，按产品是否需要移动供电取舍。", Processes: []Process{
				{Capability: "battery", Name: "电池与 PACK", Spec: "容量与形态按结构空间确定"},
				{Capability: "charger", Name: "充电与适配", Spec: "快充协议 + 温升控制"},
			}},
			{Key: "generic-assembly", Name: "组装与交付", Desc: "整机组装、测试与包装。", Processes: []Process{
				{Capability: "assembly", Name: "整机组装", Spec: "装配线体 + 直通率目标 ≥97%", Critical: true},
				{Capability: "testing", Name: "可靠性测试", Spec: "跌落、温循、老化按品类标准执行", Critical: true},
				{Capability: "packaging", Name: "包装", Spec: "彩盒 + 内托 + 说明书"},
			}},
		},
	}
}
