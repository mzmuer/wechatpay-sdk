package wechat_pay

import (
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"github.com/mzmuer/paysdk"
	"strings"
)

type Pay struct {
	AppId     string
	MchId     string
	Key       string
	SignType  string
	tlsConfig *tls.Config
	isSandBox bool
}

func NewPay(appId, mchId, key string, isSandBox bool) *Pay {
	return &Pay{
		AppId:     appId,
		MchId:     mchId,
		Key:       key,
		SignType:  paysdk.SignTypeMD5,
		isSandBox: isSandBox,
	}
}

// config
func (p *Pay) SetSignType(signType string) {
	p.SignType = signType
}

func (p *Pay) SetTLS(certFile, certKeyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, certKeyFile)
	if err != nil {
		return err
	}

	p.tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
	return nil
}

// -------------------------------------------------------------
// 创建支付订单
func (p *Pay) UnifiedOrder(req XmlMap) (XmlMap, error) {
	if req["body"] == "" ||
		req["out_trade_no"] == "" ||
		req["total_fee"] == "" ||
		req["spbill_create_ip"] == "" ||
		req["notify_url"] == "" ||
		req["trade_type"] == "" {

		return nil, fmt.Errorf("缺少必传参数")
	}

	var uri string
	if p.isSandBox {
		uri = SandboxUnifiedorderUrlSuffix
	} else {
		uri = UnifiedorderUrlSuffix
	}

	// 填充字段
	req, err := p.fillRequestData(req)
	if err != nil {
		return nil, err
	}

	// 发起请求
	resp, err := paysdk.PostXML(DomainApi+uri, req)
	if err != nil {
		return nil, err
	}

	result := XmlMap{}
	if err = xml.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if err = p.VerifyResponse(result, true); err != nil {
		return nil, err
	}

	return result, nil
}

// 退款请求
func (p *Pay) Refund(req XmlMap) (XmlMap, error) {
	if p.tlsConfig == nil {
		return nil, fmt.Errorf("before using refund must SetTLS")
	}

	if (req["transaction_id"] == "" && req["out_trade_no"] == "") ||
		req["total_fee"] == "" ||
		req["refund_fee	"] == "" {

		return nil, fmt.Errorf("缺少必传参数")
	}

	var uri string
	if p.isSandBox {
		uri = SandboxRefundUrlSuffix
	} else {
		uri = RefundUrlSuffix
	}

	// 填充字段
	req, err := p.fillRequestData(req)
	if err != nil {
		return nil, err
	}

	// 请求退款
	resp, err := paysdk.PostXMLOverTLS(DomainApi+uri, p.tlsConfig, req)
	if err != nil {
		return nil, err
	}

	result := XmlMap{}
	if err = xml.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if err = p.VerifyResponse(result, true); err != nil {
		return nil, err
	}

	return result, nil
}

// 付款到用户零钱
func (p *Pay) PromotionTransfers(req XmlMap) (XmlMap, error) {
	if p.tlsConfig == nil {
		return nil, fmt.Errorf("before using refund must SetTLS")
	}

	if req["partner_trade_no"] == "" ||
		req["openid"] == "" ||
		req["check_name"] == "" ||
		(req["check_name"] == "FORCE_CHECK" && req["re_user_name"] == "") ||
		req["amount"] == "" || req["amount"] == "0" ||
		req["desc"] == "" ||
		req["spbill_create_ip"] == "" {

		return nil, fmt.Errorf("缺少必传参数")
	}

	var uri string
	if p.isSandBox {
		uri = SandboxTransfersUrlSuffix
	} else {
		uri = TransfersUrlSuffix
	}

	// 填充字段
	req["mch_appid"] = p.AppId
	req["mchid"] = p.MchId
	req["nonce_str"] = paysdk.RandomString(24)
	sign, err := paysdk.GenerateMapSign(req, paysdk.SignTypeMD5, p.Key)
	if err != nil {
		return nil, err
	}
	req["sign"] = strings.ToUpper(sign)

	// 发起请求
	resp, err := paysdk.PostXMLOverTLS(DomainApi+uri, p.tlsConfig, req)
	if err != nil {
		return nil, err
	}

	result := XmlMap{}
	if err = xml.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if err = p.VerifyResponse(result, false); err != nil {
		return nil, err
	}

	return result, nil
}

// 查询企业付款
func (p *Pay) Gettransferinfo(partnerTradeNo string) (XmlMap, error) {
	req := XmlMap{"partner_trade_no": partnerTradeNo}

	if req["partner_trade_no"] == "" {
		return nil, fmt.Errorf("缺少必传参数")
	}

	var uri string
	if p.isSandBox {
		uri = SandboxGettransferinfoUrlSuffix
	} else {
		uri = GettransferinfoUrlSuffix
	}

	// 填充字段
	req["appid"] = p.AppId
	req["mch_id"] = p.MchId
	req["nonce_str"] = paysdk.RandomString(24)
	sign, err := paysdk.GenerateMapSign(req, p.SignType, p.Key)
	if err != nil {
		return nil, err
	}
	req["sign"] = strings.ToUpper(sign)

	// 发起请求
	resp, err := paysdk.PostXMLOverTLS(DomainApi+uri, p.tlsConfig, req)
	if err != nil {
		return nil, err
	}

	result := XmlMap{}
	if err = xml.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if err = p.VerifyResponse(result, false); err != nil {
		return nil, err
	}

	return result, nil
}

func (p *Pay) SignVerify(m XmlMap) (bool, error) {
	sign := m["sign"]
	delete(m, "sign")
	sign2, err := paysdk.GenerateMapSign(m, p.SignType, p.Key)
	return strings.ToUpper(sign2) == strings.ToUpper(sign), err
}

// --
func (p *Pay) VerifyResponse(res XmlMap, verifySign bool) error {
	if res["return_code"] != Success {
		return fmt.Errorf(res["return_code"] + "_" + res["return_msg"])
	}

	if res["result_code"] == Fail { // 业务类型错误
		return nil
	} else if res["result_code"] == Success {
		if verifySign {
			match, err := p.SignVerify(res)
			if err != nil {
				return err
			}

			if !match {
				return fmt.Errorf("sign not match[#%+v#]", res)
			}
		}
	} else { // 未知 result_code
		return fmt.Errorf(res["result_code"] + "_" + res["err_code_des"])
	}

	return nil
}

// ==========================================================
// version = 空
// appid
// mch_id
//
// version = mmpaymkttransfers
// mch_appid
// mchid

func (p *Pay) fillRequestData(m XmlMap) (XmlMap, error) {
	m["appid"] = p.AppId
	m["mch_id"] = p.MchId
	m["sign_type"] = p.SignType
	m["nonce_str"] = paysdk.RandomString(24)

	sign, err := paysdk.GenerateMapSign(m, p.SignType, p.Key)
	if err != nil {
		return nil, err
	}

	m["sign"] = strings.ToUpper(sign)

	return m, nil
}
