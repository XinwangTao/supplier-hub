package repository

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/example/supplier-hub/backend/internal/catalog"
	"github.com/example/supplier-hub/backend/internal/domain"
)

// 说明：本文件生成的是演示数据集。字段结构刻意对齐企查查/天眼查等企业征信产品的工商维度
// （法定代表人、股东、主要人员、纳税评级、专利、经营状态等），
// 方便后续把 MockCompanyProvider 换成真实数据源而无需改动上层逻辑。
const demoDataSource = "演示数据集 · 结构对齐企业征信字段"

type brand struct {
	Name   string
	Pinyin string
}

var brands = []brand{
	{"华锐", "huarui"}, {"精工", "jinggong"}, {"联创", "lianchuang"}, {"科瑞", "kerui"},
	{"鼎盛", "dingsheng"}, {"瑞达", "ruida"}, {"新拓", "xintuo"}, {"恒信", "hengxin"},
	{"智造", "zhizao"}, {"宏远", "hongyuan"}, {"启明", "qiming"}, {"森源", "senyuan"},
	{"卓越", "zhuoyue"}, {"泰科", "taike"}, {"立诚", "licheng"}, {"金瑞", "jinrui"},
	{"中科", "zhongke"}, {"远东", "yuandong"}, {"恒昌", "hengchang"}, {"铭泰", "mingtai"},
	{"华科", "huake"}, {"创力", "chuangli"}, {"弘信", "hongxin"}, {"众诚", "zhongcheng"},
	{"海纳", "haina"}, {"天工", "tiangong"}, {"优创", "youchuang"}, {"峰华", "fenghua"},
	{"广联", "guanglian"}, {"新亚", "xinya"}, {"兴森", "xingsen"}, {"汇智", "huizhi"},
	{"联德", "liande"}, {"世达", "shida"}, {"志合", "zhihe"}, {"恒泰", "hengtai"},
	{"名扬", "mingyang"}, {"锐驰", "ruichi"}, {"佳信", "jiaxin"}, {"万顺", "wanshun"},
}

var personNames = []string{
	"陈志强", "林伟", "黄建国", "吴敏", "张国华", "李振宇", "王海涛", "刘德明", "郑小军", "周文辉",
	"徐立新", "孙志刚", "马晓东", "胡建平", "朱永强", "高翔", "罗文斌", "何俊", "梁家豪", "谢鸿飞",
	"蔡文杰", "许成", "邓国平", "冯永康", "曹振华", "彭伟民", "曾志伟", "萧立群", "田文军", "董海峰",
}

var titles = []string{"执行董事兼总经理", "董事长", "总经理", "执行董事"}

// 每类能力的典型产品，用于填充 MainProducts 与经营范围。
var capabilityParts = map[string][]string{
	"cnc":        {"精密结构件", "铝制中框", "非标精密零件"},
	"mold":       {"精密塑胶模具", "压铸模具", "级进模"},
	"injection":  {"双色塑胶件", "包胶结构件", "液态硅胶件"},
	"diecasting": {"铝合金压铸件", "镁合金中框", "压铸结构件"},
	"stamping":   {"精密冲压件", "深拉伸件", "屏蔽罩"},
	"mim":        {"MIM 精密金属件", "不锈钢异形件", "钛合金小件"},
	"anodizing":  {"阳极氧化件", "硬质氧化铝件", "着色铝件"},
	"coating":    {"PVD 镀膜件", "UV 喷涂件", "IMD 装饰件"},
	"glass":      {"2.5D 玻璃盖板", "3D 热弯盖板", "强化玻璃"},
	"silicone":   {"液态硅胶件", "氟橡胶密封圈", "硅胶按键"},
	"band":       {"氟橡胶表带", "皮革表带", "声学网布"},
	"fastener":   {"精密螺丝", "弹簧件", "精密车件"},
	"laser":      {"激光切割件", "激光焊接组件", "打标加工"},
	"welding":    {"焊接结构件", "钣金机架", "折弯件"},
	"pcb":        {"HDI 电路板", "多层板", "软硬结合板"},
	"pcba":       {"PCBA 贴装", "控制主板", "功能模组"},
	"fpc":        {"柔性电路板", "软硬结合板", "FPC 天线板"},
	"display":    {"AMOLED 模组", "LCD 显示模组", "触控贴合模组"},
	"camera":     {"摄像头模组", "广角相机模组", "视觉识别模组"},
	"sensor":     {"PPG 传感模组", "IMU 模组", "激光雷达模组"},
	"battery":    {"聚合物电芯", "电池 PACK", "异形电芯"},
	"charger":    {"无线充电模组", "快充电源板", "适配器"},
	"motor":      {"无刷电机", "线性马达", "减速电机"},
	"acoustics":  {"扬声器", "MEMS 麦克风", "受话器"},
	"connector":  {"Type-C 连接器", "pogo pin", "精密线束"},
	"magnet":     {"钕铁硼磁钢", "磁组件", "充磁组件"},
	"lens":       {"光学镜头", "塑胶镜片", "镀膜镜片"},
	"antenna":    {"LDS 天线", "FPC 天线", "射频模组"},
	"heatsink":   {"热管散热模组", "VC 均温板", "导热垫"},
	"assembly":   {"整机组装", "模组组装", "成品包装"},
	"testing":    {"可靠性测试", "EMC 预测试", "防护等级测试"},
	"packaging":  {"彩盒", "纸浆内托", "说明书印刷"},
}

// 副能力：具备跨工序承接能力的企业，是网络图上产生"复用"和"替代"关系的原因。
var secondaryCapabilities = map[string][]string{
	"cnc": {"anodizing"}, "mold": {"injection"}, "injection": {"mold", "coating"},
	"diecasting": {"cnc"}, "stamping": {"mold", "fastener"}, "mim": {"cnc"},
	"anodizing": {"coating"}, "coating": {"anodizing"}, "silicone": {"band"}, "band": {"silicone"}, "laser": {"welding"}, "welding": {"stamping"},
	"pcb": {"fpc"}, "pcba": {"pcb"}, "fpc": {"pcb"}, "display": {"glass"}, "sensor": {"pcba"}, "battery": {"charger"}, "charger": {"battery"}, "motor": {"fastener"},
	"acoustics": {"silicone"}, "connector": {"stamping"}, "magnet": {"stamping"},
	"lens": {"camera"}, "antenna": {"fpc"}, "heatsink": {"stamping"},
	"assembly": {"testing", "packaging"}, "testing": {"assembly"}, "packaging": {"assembly"},
}

// 供应密度：真实产业里有些工艺供应商众多，有些则天然稀缺（如光学镜头、MIM）。
// 稀缺环节正是网络图里应该被高亮出来的"单点风险"。
var supplierDensity = map[string]int{
	"mim": 1, "lens": 1, "display": 1, "magnet": 2, "heatsink": 2, "fpc": 2, "glass": 2,
	"camera": 2, "battery": 2, "motor": 2, "acoustics": 2,
	"sensor": 2, "antenna": 2, "welding": 2, "testing": 2,
}

func supplierCountFor(code string) int {
	if n, ok := supplierDensity[code]; ok {
		return n
	}
	return 3
}

// 企业规模基线（注册资本万元 / 员工人数），按能力类别区分。
func sizeBaseline(code string) (minCap, maxCap, minEmp, maxEmp int) {
	switch code {
	case "assembly":
		return 2000, 12000, 260, 1600
	case "pcba", "pcb", "battery", "display", "camera":
		return 1500, 9000, 180, 1100
	case "testing", "packaging":
		return 500, 3500, 60, 380
	case "mold", "diecasting", "welding":
		return 800, 5000, 90, 520
	default:
		return 500, 4500, 70, 620
	}
}

func MockSuppliers() []domain.Supplier {
	now := time.Now()
	items := make([]domain.Supplier, 0, 128)
	seq := 0
	for ci, cap := range catalog.Capabilities {
		for j := 0; j < supplierCountFor(cap.Code); j++ {
			seq++
			rng := rand.New(rand.NewSource(int64(9173 + ci*131 + j*17)))
			cityName := cap.Regions[(j+ci)%len(cap.Regions)]
			city, ok := catalog.CityByName(cityName)
			if !ok {
				city = catalog.Cities[0]
			}
			b := brands[(ci*3+j)%len(brands)]
			district := city.Districts[(ci+j)%len(city.Districts)]
			cityShort := strings.TrimSuffix(city.Name, "市")
			name := fmt.Sprintf("%s%s%s有限公司", cityShort, b.Name, cap.Short)

			minCap, maxCap, minEmp, maxEmp := sizeBaseline(cap.Code)
			capital := int64(minCap + rng.Intn(maxCap-minCap))
			paid := capital * int64(58+rng.Intn(42)) / 100
			employees := minEmp + rng.Intn(maxEmp-minEmp)

			// 风险：约 8% 高风险、16% 中风险，其余低风险。
			risk, tags := domain.RiskLow, []string{"经营稳定"}
			switch {
			case rng.Intn(100) < 8:
				risk = domain.RiskHigh
				tags = []string{"行政处罚", "需现场核验"}
			case rng.Intn(100) < 16:
				risk = domain.RiskMedium
				tags = []string{"涉诉记录", "需核验"}
			default:
				if rng.Intn(100) < 28 {
					tags = []string{"专精特新", "经营稳定"}
				} else if rng.Intn(100) < 22 {
					tags = []string{"高新技术企业", "经营稳定"}
				}
			}

			caps := []string{cap.Code}
			if extra, ok := secondaryCapabilities[cap.Code]; ok && rng.Intn(100) < 45 {
				caps = append(caps, extra[rng.Intn(len(extra))])
			}
			capNames := make([]string, 0, len(caps))
			parts := make([]string, 0, 6)
			for _, code := range caps {
				if c, ok := catalog.CapabilityByCode(code); ok {
					capNames = append(capNames, c.Name)
					parts = append(parts, capabilityParts[code]...)
				}
			}
			parts = append(parts, "定制非标件")

			legalRep := personNames[(ci*3+j)%len(personNames)]
			scopeParts := make([]string, 0, 6)
			scopeParts = append(scopeParts, cap.Name+"的技术研发、生产与销售")
			scopeParts = append(scopeParts, strings.Join(cap.Processes, "、"))
			scopeParts = append(scopeParts, "模具与工装治具的设计制造")
			scopeParts = append(scopeParts, "货物进出口、技术进出口")
			scope := "一般项目：" + strings.Join(scopeParts, "；") + "。（依法须经批准的项目，经相关部门批准后方可开展经营活动）"

			area := employees * (22 + rng.Intn(20))
			revenue := int64(employees * (26 + rng.Intn(30)))

			items = append(items, domain.Supplier{
				ID:                  fmt.Sprintf("sup-%03d", seq),
				Name:                name,
				ShortName:           b.Name + cap.Short,
				CreditCode:          fmt.Sprintf("91%s%08d%s", city.RegionCode, seq*137, "XK"),
				LegalRepresentative: legalRep,
				LegalRepTitle:       titles[rng.Intn(len(titles))],
				BusinessStatus:      "存续（在营、开业、在册）",
				EnterpriseType:      []string{"有限责任公司（自然人投资或控股）", "有限责任公司（法人独资）", "股份有限公司（非上市）"}[rng.Intn(3)],
				RegisteredAddress:   fmt.Sprintf("%s%s%s%s%d号%s", city.Province, city.Name, district, catalog.Roads[rng.Intn(len(catalog.Roads))], 1+rng.Intn(200), catalog.Parks[rng.Intn(len(catalog.Parks))]),
				OfficeAddress:       fmt.Sprintf("%s%s%s%s%d号", city.Province, city.Name, district, catalog.Roads[rng.Intn(len(catalog.Roads))], 1+rng.Intn(300)),
				EstablishDate:       fmt.Sprintf("%d-%02d-%02d", 2005+rng.Intn(17), 1+rng.Intn(12), 1+rng.Intn(28)),
				RegisteredCapital:   capital,
				PaidCapital:         paid,
				TaxRating:           []string{"A", "A", "B", "B", "B", "M"}[rng.Intn(6)],
				ImportExport:        rng.Intn(100) < 55,
				HasBranches:         rng.Intn(100) < 25,
				AnnualRevenue:       revenue,
				InventionPatents:    rng.Intn(26),
				UtilityPatents:      3 + rng.Intn(90),
				Shareholders:        buildShareholders(legalRep, capital, ci, rng),
				KeyPersonnel:        buildPersonnel(legalRep, ci, rng),
				LastVerifiedAt:      now.AddDate(0, 0, -rng.Intn(120)).Format("2006-01-02"),
				BusinessScope:       scope,
				MainProducts:        dedupe(parts),
				Capabilities:        caps,
				CapabilityNames:     capNames,
				CapacityDescription: fmt.Sprintf("现有 %d 条产线、%d 台核心设备，%d 名员工（参保 %d 人），厂房 %d ㎡，支持小批量打样与规模化交付",
					2+rng.Intn(14), 8+rng.Intn(120), employees, employees*3/4, area),
				FactoryArea:    area,
				EmployeeCount:  employees,
				InsuredCount:   employees * 3 / 4,
				RndStaff:       employees * (6 + rng.Intn(12)) / 100,
				MOQ:            []int{300, 500, 1000, 2000, 3000, 5000}[rng.Intn(6)],
				LeadTimeDays:   15 + rng.Intn(31),
				Certifications: buildCerts(cap.Category, rng),
				Customers:      buildCustomers(rng),
				Province:       city.Province,
				City:           city.Name,
				District:       district,
				IsManufacturer: cap.Category != "组装服务" || rng.Intn(100) < 80,
				RiskLevel:      risk,
				RiskTags:       tags,
				ContactName:    personNames[(ci*7+j*3)%len(personNames)] + "（商务经理）",
				ContactPhone:   fmt.Sprintf("%s-%08d", city.AreaCode, 60000000+rng.Intn(9000000)),
				ContactEmail:   fmt.Sprintf("sales@%s%s.com", b.Pinyin, cap.Pinyin),
				Website:        fmt.Sprintf("https://www.%s%s.com", b.Pinyin, cap.Pinyin),
				DataSource:     demoDataSource,
				CreatedAt:      now,
				UpdatedAt:      now,
			})
		}
	}
	return items
}

func buildShareholders(legalRep string, capital int64, ci int, rng *rand.Rand) []domain.Shareholder {
	primary := 38 + rng.Intn(28)
	list := []domain.Shareholder{{
		Name: legalRep, Ratio: float64(primary), Type: "自然人", Subscribed: capital * int64(primary) / 100,
	}}
	if rng.Intn(100) < 70 {
		rest := 100 - primary
		second := rest * (45 + rng.Intn(25)) / 100
		list = append(list, domain.Shareholder{
			Name:  fmt.Sprintf("%s众诚投资合伙企业（有限合伙）", brands[(ci+11)%len(brands)].Name),
			Ratio: float64(second), Type: "企业法人", Subscribed: capital * int64(second) / 100,
		})
		list = append(list, domain.Shareholder{
			Name:  personNames[(ci+5)%len(personNames)],
			Ratio: float64(rest - second), Type: "自然人", Subscribed: capital * int64(rest-second) / 100,
		})
		return list
	}
	list = append(list, domain.Shareholder{
		Name: personNames[(ci+9)%len(personNames)], Ratio: float64(100 - primary), Type: "自然人", Subscribed: capital * int64(100-primary) / 100,
	})
	return list
}

func buildPersonnel(legalRep string, ci int, rng *rand.Rand) []domain.Personnel {
	return []domain.Personnel{
		{Name: legalRep, Title: titles[rng.Intn(len(titles))]},
		{Name: personNames[(ci+13)%len(personNames)], Title: "监事"},
		{Name: personNames[(ci+17)%len(personNames)], Title: "财务负责人"},
		{Name: personNames[(ci+21)%len(personNames)], Title: "技术总监"},
	}
}

func buildCerts(category string, rng *rand.Rand) []domain.Certification {
	year := 2027 + rng.Intn(3)
	certs := []domain.Certification{
		{Name: "ISO 9001", Issuer: "中国质量认证中心", ValidUntil: fmt.Sprintf("%d-0%d-15", year, 1+rng.Intn(9)), Scope: "生产与服务质量管理体系"},
		{Name: "ISO 14001", Issuer: "中国质量认证中心", ValidUntil: fmt.Sprintf("%d-1%d-20", year, rng.Intn(2)), Scope: "环境管理体系"},
	}
	if rng.Intn(100) < 60 {
		certs = append(certs, domain.Certification{Name: "IATF 16949", Issuer: "TÜV 莱茵", ValidUntil: fmt.Sprintf("%d-06-30", year), Scope: "汽车行业质量管理体系"})
	}
	if category == "电子" && rng.Intn(100) < 55 {
		certs = append(certs, domain.Certification{Name: "QC 080000", Issuer: "SGS", ValidUntil: fmt.Sprintf("%d-09-12", year), Scope: "有害物质过程管理"})
	}
	if rng.Intn(100) < 40 {
		certs = append(certs, domain.Certification{Name: "国家高新技术企业", Issuer: "省科学技术厅", ValidUntil: fmt.Sprintf("%d-12-31", year), Scope: "高新技术企业认定"})
	}
	return certs
}

func buildCustomers(rng *rand.Rand) []string {
	pool := []string{"国内一线手机品牌", "头部可穿戴品牌", "跨境电商头部卖家", "欧洲户外品牌", "北美智能家居品牌", "国内新能源整车厂", "日系家电品牌", "国内安防龙头"}
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	n := 2 + rng.Intn(3)
	return pool[:n]
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
