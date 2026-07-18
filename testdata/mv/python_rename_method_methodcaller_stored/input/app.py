from operator import methodcaller
import operator


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_stored_separate():
    mca = methodcaller("run")
    mcb = methodcaller("run")
    return mca(A()) + mcb(B())


def use_operator_stored_separate():
    mca = operator.methodcaller("run")
    mcb = operator.methodcaller("run")
    return mca(A()) + mcb(B())


def use_stored_only_a():
    mc = methodcaller("run")
    return mc(A())


def use_stored_multi_arg():
    mca = methodcaller("run", 0)
    mcb = methodcaller("run", 0)
    return mca(A()) + mcb(B())


def use_stored_mixed_shared():
    # Shared getter applied to both types — fail closed (keep "run").
    mc = methodcaller("run")
    return mc(A()) + mc(B())


def use_preserves_b():
    mc = methodcaller("run")
    return mc(B())
