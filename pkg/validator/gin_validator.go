/*
 * @Author: cloudyi.li
 * @Date: 2023-03-29 11:34:45
 * @LastEditTime: 2023-04-21 16:53:23
 * @LastEditors: cloudyi.li
 * @FilePath: /chatserver-api/pkg/validator/gin_validator.go
 */
// author: maxf
// date: 2022-03-28 14:44
// version: 基于github.com/go-playground/validator的校验

package validator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin/binding"
)

func LazyInitGinValidator(language string) {
	vdt := Init(language)
	binding.Validator = &customGinValidator{
		language: language,
		validate: vdt,
	}
	// 替换gin的内部validator实例
	// 将gin默认的标签 “binding” 改为 “validate”
	// 全局一致，当struct复用时可以不用在写一遍标签
	// v.SetTagName("validate")
	_ = vdt.RegisterValidation("mobile", "{0}必须为11位数字", mobileValidator)
	_ = vdt.RegisterValidation("username", "{0}不能包含admin", usernameValidator)
}

type sliceValidateError []error

func (err sliceValidateError) Error() string {
	var errMsgs []string
	for i, e := range err {
		if e == nil {
			continue
		}
		errMsgs = append(errMsgs, fmt.Sprintf("[%d]: %s", i, e.Error()))
	}
	return strings.Join(errMsgs, "\n")
}

// customGinValidator 用来替换gin默认的校验器，必须实现binding.StructValidator
// 代码参考 github.com/gin-gonic/gin/binding/default_validator.go
type customGinValidator struct {
	language string
	validate Validator
}

var _ binding.StructValidator = &customGinValidator{}

// ValidateStruct receives any kind of type, but only performed struct or pointer to struct type.
func (v *customGinValidator) ValidateStruct(obj interface{}) error {
	if obj == nil {
		return nil
	}

	value := reflect.ValueOf(obj)
	switch value.Kind() {
	case reflect.Ptr:
		return v.ValidateStruct(value.Elem().Interface())
	case reflect.Struct:
		return v.validateStruct(obj)
	case reflect.Slice, reflect.Array:
		count := value.Len()
		validateRet := make(sliceValidateError, 0)
		for i := 0; i < count; i++ {
			if err := v.ValidateStruct(value.Index(i).Interface()); err != nil {
				validateRet = append(validateRet, err)
			}
		}
		if len(validateRet) == 0 {
			return nil
		}
		return validateRet
	default:
		return nil
	}
}

// validateStruct receives struct type
func (v *customGinValidator) validateStruct(obj interface{}) error {
	v.lazyInit(v.language)
	return v.validate.ValidStruct(obj)
}

func (v *customGinValidator) Engine() interface{} {
	v.lazyInit(v.language)
	return v.validate.ValidatorEngine()
}

func (v *customGinValidator) lazyInit(language string) {
	v.validate = Init(language)
}
