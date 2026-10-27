package repository

import (
	"fmt"
	"github.com/example/supplier-hub/backend/internal/domain"
	"time"
)

func MockSuppliers() []domain.Supplier {
	places := []struct{ province, city string }{{"广东省", "东莞市"}, {"广东省", "深圳市"}, {"江苏省", "苏州市"}, {"浙江省", "宁波市"}, {"浙江省", "杭州市"}, {"江苏省", "无锡市"}, {"山东省", "青岛市"}, {"福建省", "厦门市"}}
	names := []string{"华锐", "精工", "联创", "科瑞", "鼎盛", "瑞达", "新拓", "恒信", "智造", "宏远", "启明", "森源", "卓越"}
	products := []string{"精密注塑件", "工业连接器", "铝合金结构件", "自动化设备", "电子线束", "五金冲压件", "新能源零部件", "医疗器械配件"}
	items := make([]domain.Supplier, 0, 104)
	now := time.Now()
	for i := 0; i < 104; i++ {
		place, product := places[i%len(places)], products[i%len(products)]
		capital, employees := int64(300+(i*137)%9700), 35+(i*83)%1460
		risk, tags := domain.RiskLow, []string{"经营稳定"}
		if i%11 == 0 {
			risk, tags = domain.RiskMedium, []string{"涉诉记录", "需核验"}
		}
		if i%29 == 0 {
			risk, tags = domain.RiskHigh, []string{"行政处罚", "重点关注"}
		}
		certs := []string{"ISO 9001", "ISO 14001"}
		if i%4 == 0 {
			certs = append(certs, "IATF 16949")
		}
		items = append(items, domain.Supplier{
			ID: fmt.Sprintf("sup-%03d", i+1), Name: fmt.Sprintf("%s%s%s有限公司", place.city, names[i%len(names)], product), CreditCode: fmt.Sprintf("9144%014d", 10000000000000+i),
			BusinessScope: fmt.Sprintf("%s研发、生产、制造、销售；精密加工与模具设计；供应链服务", product), RegisteredCapital: capital, PaidCapital: capital * int64(60+(i%41)) / 100, EmployeeCount: employees,
			EstablishDate: fmt.Sprintf("%d-%02d-%02d", 2008+i%15, i%12+1, i%27+1), Province: place.province, City: place.city, IsManufacturer: i%9 != 0, RiskLevel: risk, RiskTags: tags, Certifications: certs,
			MainProducts: []string{product, "定制非标件"}, CapacityDescription: fmt.Sprintf("拥有%d条核心产线，支持小批量打样与规模化交付", 2+i%12), CreatedAt: now, UpdatedAt: now,
		})
	}
	return items
}
