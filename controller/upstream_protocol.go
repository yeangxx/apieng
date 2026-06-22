package controller

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func buildUpstreamProtocolTemplateFromRequest(req dto.UpstreamProtocolTemplateRequest) *model.UpstreamProtocolTemplate {
	return &model.UpstreamProtocolTemplate{
		Name:                 req.Name,
		ProviderLabel:        req.ProviderLabel,
		Protocol:             req.Protocol,
		ClientPath:           req.ClientPath,
		EndpointPath:         req.EndpointPath,
		AuthType:             req.AuthType,
		AuthHeader:           req.AuthHeader,
		AuthQueryName:        req.AuthQueryName,
		DefaultHeaders:       req.DefaultHeaders,
		DefaultQuery:         req.DefaultQuery,
		DefaultBodyOverrides: req.DefaultBodyOverrides,
		RequestConverter:     req.RequestConverter,
		ResponseConverter:    req.ResponseConverter,
		StreamConverter:      req.StreamConverter,
		DetectRules:          req.DetectRules,
		Enabled:              req.Enabled,
		Sort:                 req.Sort,
	}
}

func buildUpstreamProtocolTemplateResponse(template *model.UpstreamProtocolTemplate) dto.UpstreamProtocolTemplateResponse {
	return dto.UpstreamProtocolTemplateResponse{
		Id:                   template.Id,
		Name:                 template.Name,
		ProviderLabel:        template.ProviderLabel,
		Protocol:             template.Protocol,
		ClientPath:           template.ClientPath,
		EndpointPath:         template.EndpointPath,
		AuthType:             template.AuthType,
		AuthHeader:           template.AuthHeader,
		AuthQueryName:        template.AuthQueryName,
		DefaultHeaders:       template.DefaultHeaders,
		DefaultQuery:         template.DefaultQuery,
		DefaultBodyOverrides: template.DefaultBodyOverrides,
		RequestConverter:     template.RequestConverter,
		ResponseConverter:    template.ResponseConverter,
		StreamConverter:      template.StreamConverter,
		DetectRules:          template.DetectRules,
		Enabled:              template.Enabled,
		Sort:                 template.Sort,
		CreatedAt:            template.CreatedAt,
		UpdatedAt:            template.UpdatedAt,
	}
}

func buildUpstreamProtocolTemplateResponses(templates []*model.UpstreamProtocolTemplate) []dto.UpstreamProtocolTemplateResponse {
	responses := make([]dto.UpstreamProtocolTemplateResponse, 0, len(templates))
	for _, template := range templates {
		responses = append(responses, buildUpstreamProtocolTemplateResponse(template))
	}
	return responses
}

func GetUpstreamProtocolConverters(c *gin.Context) {
	converters := model.ListUpstreamProtocolConverters()
	common.ApiSuccess(c, gin.H{
		"items": converters,
		"total": len(converters),
	})
}

func GetUpstreamProtocolTemplates(c *gin.Context) {
	templates, err := model.ListUpstreamProtocolTemplates(false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items": buildUpstreamProtocolTemplateResponses(templates),
		"total": len(templates),
	})
}

func AdminGetUpstreamProtocolTemplates(c *gin.Context) {
	templates, err := model.ListUpstreamProtocolTemplates(true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items": buildUpstreamProtocolTemplateResponses(templates),
		"total": len(templates),
	})
}

func AdminCreateUpstreamProtocolTemplate(c *gin.Context) {
	var req dto.UpstreamProtocolTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	template := buildUpstreamProtocolTemplateFromRequest(req)
	if err := template.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildUpstreamProtocolTemplateResponse(template))
}

func AdminUpdateUpstreamProtocolTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, errors.New("invalid template id"))
		return
	}
	var req dto.UpstreamProtocolTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	existing, err := model.GetUpstreamProtocolTemplateByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	template := buildUpstreamProtocolTemplateFromRequest(req)
	template.Id = existing.Id
	if err := template.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	updated, err := model.GetUpstreamProtocolTemplateByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildUpstreamProtocolTemplateResponse(updated))
}

func AdminDeleteUpstreamProtocolTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, errors.New("invalid template id"))
		return
	}
	if err := model.DeleteUpstreamProtocolTemplateByID(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
