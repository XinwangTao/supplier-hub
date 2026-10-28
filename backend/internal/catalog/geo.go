package catalog

import "strings"

// City 是生成供应商地址与工商信息所需的行政区划数据。
type City struct {
	Province   string
	Name       string
	Districts  []string
	AreaCode   string // 固话区号
	RegionCode string // 行政区划代码前 6 位，用于统一社会信用代码
}

var Cities = []City{
	{Province: "广东省", Name: "东莞市", Districts: []string{"长安镇", "松山湖", "虎门镇", "塘厦镇"}, AreaCode: "0769", RegionCode: "441900"},
	{Province: "广东省", Name: "深圳市", Districts: []string{"宝安区", "龙华区", "光明区", "坪山区"}, AreaCode: "0755", RegionCode: "440300"},
	{Province: "广东省", Name: "惠州市", Districts: []string{"仲恺高新区", "惠阳区", "博罗县"}, AreaCode: "0752", RegionCode: "441300"},
	{Province: "广东省", Name: "佛山市", Districts: []string{"顺德区", "南海区", "三水区"}, AreaCode: "0757", RegionCode: "440600"},
	{Province: "广东省", Name: "珠海市", Districts: []string{"金湾区", "香洲区", "斗门区"}, AreaCode: "0756", RegionCode: "440400"},
	{Province: "广东省", Name: "中山市", Districts: []string{"火炬开发区", "小榄镇", "三乡镇"}, AreaCode: "0760", RegionCode: "442000"},
	{Province: "广东省", Name: "广州市", Districts: []string{"番禺区", "花都区", "增城区"}, AreaCode: "020", RegionCode: "440100"},
	{Province: "江苏省", Name: "苏州市", Districts: []string{"吴中区", "相城区", "苏州工业园区"}, AreaCode: "0512", RegionCode: "320500"},
	{Province: "江苏省", Name: "昆山市", Districts: []string{"玉山镇", "周市镇", "花桥镇"}, AreaCode: "0512", RegionCode: "320583"},
	{Province: "江苏省", Name: "无锡市", Districts: []string{"锡山区", "惠山区", "新吴区"}, AreaCode: "0510", RegionCode: "320200"},
	{Province: "江苏省", Name: "常州市", Districts: []string{"武进区", "新北区", "金坛区"}, AreaCode: "0519", RegionCode: "320400"},
	{Province: "浙江省", Name: "宁波市", Districts: []string{"北仑区", "鄞州区", "余姚市"}, AreaCode: "0574", RegionCode: "330200"},
	{Province: "浙江省", Name: "杭州市", Districts: []string{"余杭区", "萧山区", "钱塘区"}, AreaCode: "0571", RegionCode: "330100"},
	{Province: "浙江省", Name: "温州市", Districts: []string{"瓯海区", "瑞安市", "乐清市"}, AreaCode: "0577", RegionCode: "330300"},
	{Province: "山东省", Name: "青岛市", Districts: []string{"城阳区", "黄岛区", "即墨区"}, AreaCode: "0532", RegionCode: "370200"},
	{Province: "福建省", Name: "厦门市", Districts: []string{"集美区", "海沧区", "同安区"}, AreaCode: "0592", RegionCode: "350200"},
	{Province: "福建省", Name: "泉州市", Districts: []string{"晋江市", "南安市", "石狮市"}, AreaCode: "0595", RegionCode: "350500"},
	{Province: "上海市", Name: "上海市", Districts: []string{"嘉定区", "松江区", "浦东新区"}, AreaCode: "021", RegionCode: "310100"},
	{Province: "重庆市", Name: "重庆市", Districts: []string{"璧山区", "渝北区", "江津区"}, AreaCode: "023", RegionCode: "500100"},
	{Province: "天津市", Name: "天津市", Districts: []string{"西青区", "北辰区", "滨海新区"}, AreaCode: "022", RegionCode: "120100"},
	{Province: "江西省", Name: "赣州市", Districts: []string{"章贡区", "南康区", "赣县区"}, AreaCode: "0797", RegionCode: "360700"},
}

var cityIndex = func() map[string]City {
	index := make(map[string]City, len(Cities))
	for _, c := range Cities {
		index[c.Name] = c
	}
	return index
}()

func CityByName(name string) (City, bool) {
	c, ok := cityIndex[name]
	return c, ok
}

// Roads 与园区后缀用于生成像样的注册地址。
var Roads = []string{"工业大道", "科技路", "环城北路", "兴华路", "振安路", "长安二路", "创新大道", "金湾路", "东升路", "高新南七道"}

var Parks = []string{"智造产业园", "高新科技园", "精密制造基地", "电子信息产业园", "智能装备园", "产业孵化基地"}

var provinceNames = []string{
	"北京市", "上海市", "天津市", "重庆市",
	"广东省", "江苏省", "浙江省", "福建省", "山东省", "河南省", "河北省", "湖北省",
	"湖南省", "安徽省", "四川省", "江西省", "陕西省", "辽宁省", "吉林省", "黑龙江省",
	"云南省", "贵州省", "广西壮族自治区", "内蒙古自治区", "山西省", "甘肃省", "海南省",
	"宁夏回族自治区", "青海省", "新疆维吾尔自治区", "西藏自治区", "台湾省",
	"香港特别行政区", "澳门特别行政区",
}

// SplitAddress 从真实注册地址里拆出省/市/区县。
// 真实数据的地址写法千奇百怪，这里只做保守解析：拆不出来就留空，
// 让前端显示原始地址，而不是猜一个看起来合理的行政级别。
func SplitAddress(address string) (province, city, district string) {
	rest := address
	for _, name := range provinceNames {
		if idx := strings.Index(rest, name); idx >= 0 {
			province = name
			rest = rest[idx+len(name):]
			break
		}
	}

	// 市：优先匹配已知城市，再退回"第一个 市"的简单规则
	for _, c := range Cities {
		if idx := strings.Index(rest, c.Name); idx >= 0 && idx <= 6 {
			city = c.Name
			rest = rest[idx+len(c.Name):]
			break
		}
	}
	if city == "" {
		if idx := strings.Index(rest, "市"); idx > 0 && idx <= 6 {
			city = rest[:idx+len("市")]
			rest = rest[idx+len("市"):]
		}
	}

	for _, suffix := range []string{"区", "县", "镇"} {
		if idx := strings.Index(rest, suffix); idx > 0 && idx <= 8 {
			district = rest[:idx+len(suffix)]
			break
		}
	}
	return province, city, district
}
