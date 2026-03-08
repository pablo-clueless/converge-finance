package auth

const (
	PermGLAccountRead    = "gl:account:read"
	PermGLAccountCreate  = "gl:account:create"
	PermGLAccountUpdate  = "gl:account:update"
	PermGLAccountDelete  = "gl:account:delete"
	PermGLJournalRead    = "gl:journal:read"
	PermGLJournalCreate  = "gl:journal:create"
	PermGLJournalPost    = "gl:journal:post"
	PermGLJournalReverse = "gl:journal:reverse"
	PermGLJournalApprove = "gl:journal:approve"
	PermGLPeriodManage   = "gl:period:manage"
	PermGLReportView     = "gl:report:view"
	PermAPVendorRead     = "ap:vendor:read"
	PermAPVendorCreate   = "ap:vendor:create"
	PermAPVendorUpdate   = "ap:vendor:update"
	PermAPVendorDelete   = "ap:vendor:delete"
	PermAPInvoiceRead    = "ap:invoice:read"
	PermAPInvoiceCreate  = "ap:invoice:create"
	PermAPInvoiceApprove = "ap:invoice:approve"
	PermAPPaymentRead    = "ap:payment:read"
	PermAPPaymentCreate  = "ap:payment:create"
	PermAPPaymentApprove = "ap:payment:approve"
	PermAPPaymentExecute = "ap:payment:execute"
	PermARCustomerRead   = "ar:customer:read"
	PermARCustomerCreate = "ar:customer:create"
	PermARCustomerUpdate = "ar:customer:update"
	PermARCustomerDelete = "ar:customer:delete"
	PermARInvoiceRead    = "ar:invoice:read"
	PermARInvoiceCreate  = "ar:invoice:create"
	PermARInvoiceSend    = "ar:invoice:send"
	PermARReceiptRead    = "ar:receipt:read"
	PermARReceiptCreate  = "ar:receipt:create"
	PermARCreditManage   = "ar:credit:manage"
	PermARDunningManage  = "ar:dunning:manage"
	PermEntityRead       = "entity:read"
	PermEntityCreate     = "entity:create"
	PermEntityUpdate     = "entity:update"
	PermEntityDelete     = "entity:delete"
	PermUserManage       = "user:manage"
	PermRoleManage       = "role:manage"
	PermAuditView        = "audit:view"
	PermSettingsManage   = "settings:manage"
)

type Role string

const (
	RoleAdmin      Role = "admin"
	RoleAccountant Role = "accountant"
	RoleAPClerk    Role = "ap_clerk"
	RoleARClerk    Role = "ar_clerk"
	RoleController Role = "controller"
	RoleViewer     Role = "viewer"
)

var rolePermissions = map[Role][]string{
	RoleAdmin: {
		PermGLAccountRead, PermGLAccountCreate, PermGLAccountUpdate, PermGLAccountDelete,
		PermGLJournalRead, PermGLJournalCreate, PermGLJournalPost, PermGLJournalReverse, PermGLJournalApprove,
		PermGLPeriodManage, PermGLReportView,
		PermAPVendorRead, PermAPVendorCreate, PermAPVendorUpdate, PermAPVendorDelete,
		PermAPInvoiceRead, PermAPInvoiceCreate, PermAPInvoiceApprove,
		PermAPPaymentRead, PermAPPaymentCreate, PermAPPaymentApprove, PermAPPaymentExecute,
		PermARCustomerRead, PermARCustomerCreate, PermARCustomerUpdate, PermARCustomerDelete,
		PermARInvoiceRead, PermARInvoiceCreate, PermARInvoiceSend,
		PermARReceiptRead, PermARReceiptCreate, PermARCreditManage, PermARDunningManage,
		PermEntityRead, PermEntityCreate, PermEntityUpdate, PermEntityDelete,
		PermUserManage, PermRoleManage, PermAuditView, PermSettingsManage,
	},
	RoleAccountant: {
		PermGLAccountRead, PermGLAccountCreate, PermGLAccountUpdate,
		PermGLJournalRead, PermGLJournalCreate, PermGLJournalPost, PermGLJournalReverse,
		PermGLPeriodManage, PermGLReportView,
		PermAPVendorRead, PermAPInvoiceRead, PermAPPaymentRead,
		PermARCustomerRead, PermARInvoiceRead, PermARReceiptRead,
		PermEntityRead, PermAuditView,
	},
	RoleController: {
		PermGLAccountRead, PermGLJournalRead, PermGLJournalApprove, PermGLReportView,
		PermAPVendorRead, PermAPInvoiceRead, PermAPInvoiceApprove,
		PermAPPaymentRead, PermAPPaymentApprove,
		PermARCustomerRead, PermARInvoiceRead, PermARReceiptRead,
		PermARCreditManage,
		PermEntityRead, PermAuditView,
	},
	RoleAPClerk: {
		PermGLAccountRead, PermGLReportView,
		PermAPVendorRead, PermAPVendorCreate, PermAPVendorUpdate,
		PermAPInvoiceRead, PermAPInvoiceCreate,
		PermAPPaymentRead, PermAPPaymentCreate,
		PermEntityRead,
	},
	RoleARClerk: {
		PermGLAccountRead, PermGLReportView,
		PermARCustomerRead, PermARCustomerCreate, PermARCustomerUpdate,
		PermARInvoiceRead, PermARInvoiceCreate, PermARInvoiceSend,
		PermARReceiptRead, PermARReceiptCreate,
		PermARDunningManage,
		PermEntityRead,
	},
	RoleViewer: {
		PermGLAccountRead, PermGLJournalRead, PermGLReportView,
		PermAPVendorRead, PermAPInvoiceRead, PermAPPaymentRead,
		PermARCustomerRead, PermARInvoiceRead, PermARReceiptRead,
		PermEntityRead,
	},
}

func GetPermissionsForRole(role Role) []string {
	if perms, ok := rolePermissions[role]; ok {
		return perms
	}
	return nil
}

func GetPermissionsForRoles(roles []string) []string {
	permSet := make(map[string]bool)
	for _, r := range roles {
		for _, p := range GetPermissionsForRole(Role(r)) {
			permSet[p] = true
		}
	}

	perms := make([]string, 0, len(permSet))
	for p := range permSet {
		perms = append(perms, p)
	}
	return perms
}

func HasPermission(permissions []string, required string) bool {
	for _, p := range permissions {
		if p == required {
			return true
		}
	}
	return false
}

func HasAnyPermission(permissions []string, required []string) bool {
	for _, r := range required {
		if HasPermission(permissions, r) {
			return true
		}
	}
	return false
}

func HasAllPermissions(permissions []string, required []string) bool {
	for _, r := range required {
		if !HasPermission(permissions, r) {
			return false
		}
	}
	return true
}

func IsValidRole(role string) bool {
	_, ok := rolePermissions[Role(role)]
	return ok
}

func ListRoles() []Role {
	return []Role{
		RoleAdmin,
		RoleAccountant,
		RoleController,
		RoleAPClerk,
		RoleARClerk,
		RoleViewer,
	}
}
