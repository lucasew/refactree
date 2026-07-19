from operator import methodcaller
import operator


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    item: A

    def __init__(self, item: A):
        self.item = item

    def get(self) -> A:
        return self.item


class BoxB:
    item: B

    def __init__(self, item: B):
        self.item = item

    def get(self) -> B:
        return self.item


def use_mc_method_return(ba: BoxA, bb: BoxB) -> int:
    return methodcaller("run")(ba.get()) + methodcaller("run")(bb.get())


def use_op_mc_method_return(ba: BoxA, bb: BoxB) -> int:
    return operator.methodcaller("run")(ba.get()) + operator.methodcaller("run")(bb.get())


def use_mc_assign(ba: BoxA, bb: BoxB) -> int:
    xa = ba.get()
    xb = bb.get()
    return methodcaller("run")(xa) + methodcaller("run")(xb)


def use_mc_stored(ba: BoxA, bb: BoxB) -> int:
    mca = methodcaller("run")
    mcb = methodcaller("run")
    return mca(ba.get()) + mcb(bb.get())


def use_mc_stored_mixed(ba: BoxA, bb: BoxB) -> int:
    # Shared getter applied to both — fail closed
    mc = methodcaller("run")
    return mc(ba.get()) + mc(bb.get())


def use_mc_multi_arg(ba: BoxA, bb: BoxB) -> int:
    return methodcaller("run", 0)(ba.get()) + methodcaller("run", 0)(bb.get())


def use_class() -> int:
    return methodcaller("run")(A()) + methodcaller("run")(B())


def use_preserves_b(bb: BoxB) -> int:
    return methodcaller("run")(bb.get()) + methodcaller("run")(B())
