package main

import (
	"github.com/zhanghui/juzheng/internal/singboxcfg"
	"github.com/zhanghui/juzheng/internal/subscriptions"
)

// subscription_service.go：独立的订阅管理 Wails service（L3 薄壳）。
//
// 从 SingBoxService 拆出，所有订阅方法委托 internal/subscriptions（L1）。
// 前端通过 subscriptionservice binding 调用。

// SubscriptionService 是订阅管理 Wails service。
// 从 SingBoxService 拆出，专注于订阅 CRUD/导入导出/活动切换。
type SubscriptionService struct{}

// 订阅类型别名（SubscriptionMeta 归 subscriptionservice 暴露）。
// SubscriptionInfo 由 singbox_types.go 统一暴露（收入 models.ts）。
type SubscriptionMeta = subscriptions.Meta

func (s *SubscriptionService) ListSubscriptions() ([]SubscriptionMeta, error) {
	return subscriptions.List()
}
func (s *SubscriptionService) GetSubscription(name string) (string, error) {
	return subscriptions.Get(name)
}
func (s *SubscriptionService) SetSubscription(name, content string) error {
	return subscriptions.Set(name, content)
}
func (s *SubscriptionService) DeleteSubscription(name string) error {
	return subscriptions.Delete(name)
}
func (s *SubscriptionService) RenameSubscription(old, newName string) error {
	return subscriptions.Rename(old, newName)
}
func (s *SubscriptionService) SetActiveSubscription(name string) error {
	return subscriptions.SetActive(name)
}
func (s *SubscriptionService) GetActiveSubscription() string {
	return subscriptions.GetActive()
}
func (s *SubscriptionService) CreateSubscriptionFromTemplate(name, source string) error {
	return subscriptions.CreateFromTemplate(name, source)
}
func (s *SubscriptionService) ImportSubscriptionFromFile(name, path string) error {
	return subscriptions.ImportFromFile(name, path)
}
func (s *SubscriptionService) ImportSubscriptionFromURL(name, url string) error {
	return subscriptions.ImportFromURL(name, url)
}
func (s *SubscriptionService) RefreshSubscriptionInfo(name string) error {
	return subscriptions.RefreshInfo(name)
}
func (s *SubscriptionService) ExportSubscription(name string) (string, error) {
	return subscriptions.Export(name)
}

// meta/卡片配置也归订阅服务管（活动订阅 + 卡片显隐，都是 meta.json 内容）。
func (s *SubscriptionService) GetMeta() (AppMeta, error) { return singboxcfg.ReadMeta() }
func (s *SubscriptionService) SetMeta(m AppMeta) error   { return singboxcfg.WriteMeta(m) }

func NewSubscriptionService() *SubscriptionService { return &SubscriptionService{} }
