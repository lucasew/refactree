class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_getattr_call():
    return getattr(A(), "execute")() + getattr(B(), "run")()


def use_getattr_assign():
    fa = getattr(A(), "execute")
    fb = getattr(B(), "run")
    return fa() + fb()


def use_preserves_b():
    return getattr(B(), "run")()
