package captcha

import "strings"

// TaskType describes one 2captcha task type: its required/optional fields and where the answer
// lives in the solution object. It powers captcha_list_types, captcha_solve's pre-validation,
// and the CLI's per-family subcommands.
type TaskType struct {
	Name         string
	Family       string
	Description  string
	Required     []string
	Optional     []string
	SolutionKeys []string // checked in order; first present key becomes the convenience "token"
}

// Catalog lists the 2captcha task types this server has first-class knowledge of. It is not
// exhaustive — captcha_solve forwards any type when the caller sets AllowUnknownType, so new
// 2captcha task types work without a release.
var Catalog = []TaskType{
	{
		Name: "RecaptchaV2TaskProxyless", Family: "recaptcha",
		Description:  "Google reCAPTCHA v2, solved from 2captcha's own IP pool.",
		Required:     []string{"websiteURL", "websiteKey"},
		Optional:     []string{"isInvisible", "userAgent", "cookies", "dataS"},
		SolutionKeys: []string{"gRecaptchaResponse", "token"},
	},
	{
		Name: "RecaptchaV2Task", Family: "recaptcha",
		Description:  "Google reCAPTCHA v2, solved through a caller-supplied proxy.",
		Required:     []string{"websiteURL", "websiteKey", "proxyType", "proxyAddress", "proxyPort"},
		Optional:     []string{"isInvisible", "userAgent", "cookies", "dataS", "proxyLogin", "proxyPassword"},
		SolutionKeys: []string{"gRecaptchaResponse", "token"},
	},
	{
		Name: "RecaptchaV2EnterpriseTaskProxyless", Family: "recaptcha",
		Description:  "Google reCAPTCHA v2 Enterprise, solved from 2captcha's own IP pool.",
		Required:     []string{"websiteURL", "websiteKey"},
		Optional:     []string{"isInvisible", "userAgent", "cookies", "enterprisePayload", "apiDomain"},
		SolutionKeys: []string{"gRecaptchaResponse", "token"},
	},
	{
		Name: "RecaptchaV3TaskProxyless", Family: "recaptcha",
		Description:  "Google reCAPTCHA v3, solved from 2captcha's own IP pool.",
		Required:     []string{"websiteURL", "websiteKey"},
		Optional:     []string{"pageAction", "minScore", "isEnterprise", "apiDomain"},
		SolutionKeys: []string{"gRecaptchaResponse", "token"},
	},
	{
		Name: "RecaptchaV3EnterpriseTaskProxyless", Family: "recaptcha",
		Description:  "Google reCAPTCHA v3 Enterprise, solved from 2captcha's own IP pool.",
		Required:     []string{"websiteURL", "websiteKey"},
		Optional:     []string{"pageAction", "minScore", "apiDomain"},
		SolutionKeys: []string{"gRecaptchaResponse", "token"},
	},
	{
		Name: "TurnstileTaskProxyless", Family: "turnstile",
		Description:  "Cloudflare Turnstile, solved from 2captcha's own IP pool.",
		Required:     []string{"websiteURL", "websiteKey"},
		Optional:     []string{"action", "cdata", "pagedata", "userAgent"},
		SolutionKeys: []string{"token"},
	},
	{
		Name: "TurnstileTask", Family: "turnstile",
		Description:  "Cloudflare Turnstile, solved through a caller-supplied proxy.",
		Required:     []string{"websiteURL", "websiteKey", "proxyType", "proxyAddress", "proxyPort"},
		Optional:     []string{"action", "cdata", "pagedata", "userAgent", "proxyLogin", "proxyPassword"},
		SolutionKeys: []string{"token"},
	},
	{
		Name: "HCaptchaTaskProxyless", Family: "hcaptcha",
		Description:  "hCaptcha, solved from 2captcha's own IP pool.",
		Required:     []string{"websiteURL", "websiteKey"},
		Optional:     []string{"isInvisible", "userAgent"},
		SolutionKeys: []string{"gRecaptchaResponse", "token"},
	},
	{
		Name: "HCaptchaTask", Family: "hcaptcha",
		Description:  "hCaptcha, solved through a caller-supplied proxy.",
		Required:     []string{"websiteURL", "websiteKey", "proxyType", "proxyAddress", "proxyPort"},
		Optional:     []string{"isInvisible", "userAgent", "proxyLogin", "proxyPassword"},
		SolutionKeys: []string{"gRecaptchaResponse", "token"},
	},
	{
		Name: "FunCaptchaTaskProxyless", Family: "funcaptcha",
		Description:  "Arkose Labs FunCaptcha, solved from 2captcha's own IP pool.",
		Required:     []string{"websiteURL", "websitePublicKey"},
		Optional:     []string{"funcaptchaApiJSSubdomain", "data", "userAgent"},
		SolutionKeys: []string{"token"},
	},
	{
		Name: "GeeTestTaskProxyless", Family: "geetest",
		Description:  "GeeTest v3/v4, solved from 2captcha's own IP pool.",
		Required:     []string{"websiteURL", "gt"},
		Optional:     []string{"challenge", "geetestApiServerSubdomain", "version", "captchaId", "initParameters", "userAgent"},
		SolutionKeys: []string{"challenge", "validate", "seccode", "captcha_id", "lot_number", "pass_token", "gen_time", "captcha_output"},
	},
	{
		Name: "AmazonTaskProxyless", Family: "amazon",
		Description:  "Amazon WAF captcha, solved from 2captcha's own IP pool.",
		Required:     []string{"websiteURL", "websiteKey", "iv", "context"},
		Optional:     []string{"challengeScript", "captchaScript", "userAgent"},
		SolutionKeys: []string{"captcha_voucher", "existing_token"},
	},
	{
		Name: "ImageToTextTask", Family: "image",
		Description:  "Image-based captcha (text, digits, or phrase) sent as a base64 body.",
		Required:     []string{"body"},
		Optional:     []string{"phrase", "case", "numeric", "math", "minLength", "maxLength", "comment", "textinstructions"},
		SolutionKeys: []string{"text"},
	},
	{
		Name: "TextCaptchaTask", Family: "image",
		Description:  "Text-only question/answer captcha (\"what is 2+3?\").",
		Required:     []string{"comment"},
		Optional:     nil,
		SolutionKeys: []string{"text"},
	},
	{
		Name: "CoordinatesTask", Family: "image",
		Description:  "Click-the-image-region captcha; solution is a list of x/y coordinates.",
		Required:     []string{"body"},
		Optional:     []string{"comment", "minClicks", "maxClicks"},
		SolutionKeys: []string{"points", "coordinates"},
	},
	{
		Name: "GridTask", Family: "image",
		Description:  "Grid-based image-click captcha; solution is a list of selected cell numbers.",
		Required:     []string{"body", "rows", "columns"},
		Optional:     []string{"comment", "canNotSelectMultiple"},
		SolutionKeys: []string{"click", "points"},
	},
	{
		Name: "RotateTask", Family: "image",
		Description:  "Rotate-the-image-to-upright captcha; solution is a rotation angle.",
		Required:     []string{"body"},
		Optional:     []string{"comment", "angle"},
		SolutionKeys: []string{"rotate"},
	},
}

// ByName looks up a catalog entry (case-sensitive, as 2captcha type names are).
func ByName(name string) (TaskType, bool) {
	for _, t := range Catalog {
		if t.Name == name {
			return t, true
		}
	}
	return TaskType{}, false
}

// ByFamily returns catalog entries whose Family matches, or the full catalog if family is empty.
func ByFamily(family string) []TaskType {
	if family == "" {
		return Catalog
	}
	var out []TaskType
	for _, t := range Catalog {
		if strings.EqualFold(t.Family, family) {
			out = append(out, t)
		}
	}
	return out
}

// MissingRequired returns the subset of t.Required not present as a key in task, preserving order.
func MissingRequired(t TaskType, task map[string]any) []string {
	var missing []string
	for _, field := range t.Required {
		if _, ok := task[field]; !ok {
			missing = append(missing, field)
		}
	}
	return missing
}

// SuggestNames returns catalog names sharing a case-insensitive fragment with name, for
// "did you mean" hints when an unknown type is requested.
func SuggestNames(name string) []string {
	lower := strings.ToLower(name)
	var out []string
	for _, t := range Catalog {
		if strings.Contains(strings.ToLower(t.Name), lower) || strings.Contains(lower, strings.ToLower(t.Family)) {
			out = append(out, t.Name)
		}
	}
	return out
}
