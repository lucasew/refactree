class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_getattr_call():
    return getattr(A(), "run")() + getattr(B(), "run")()


def use_getattr_assign():
    fa = getattr(A(), "run")
    fb = getattr(B(), "run")
    return fa() + fb()


def use_preserves_b():
    return getattr(B(), "run")()
