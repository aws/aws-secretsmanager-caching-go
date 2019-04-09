package secretcache


type baseError struct {
	Message string
}

type VersionNotFoundError struct {
	baseError
}

func (v *VersionNotFoundError) Error() string {
	return v.Message
}

type InvalidConfigError struct {
	baseError
}

func (i *InvalidConfigError) Error() string {
	return i.Message
}

type InvalidOperationError struct {
	baseError
}

func (i *InvalidOperationError) Error() string {
	return i.Message
}

