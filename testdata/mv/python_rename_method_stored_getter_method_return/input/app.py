from operator import itemgetter, attrgetter
from dataclasses import dataclass
import operator


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    a: A

    def __init__(self, a: A):
        self.a = a

    def get(self) -> A:
        return self.a


class BoxB:
    b: B

    def __init__(self, b: B):
        self.b = b

    def get(self) -> B:
        return self.b


@dataclass
class Wrap:
    a: A

    def get(self) -> "Wrap":
        return self


@dataclass
class WrapB:
    a: B

    def get(self) -> "WrapB":
        return self


def use_ig(xa: BoxA, xb: BoxB) -> int:
    gi = itemgetter(0)
    return gi([xa.get()]).run() + gi([xb.get()]).run()


def use_ig_op(ya: BoxA, yb: BoxB) -> int:
    gi = operator.itemgetter(0)
    return gi([ya.get()]).run() + gi([yb.get()]).run()


def use_ig_assign(za: BoxA, zb: BoxB) -> int:
    gi = itemgetter(0)
    a = gi([za.get()])
    b = gi([zb.get()])
    return a.run() + b.run()


def use_ig_key(ka: BoxA, kb: BoxB) -> int:
    gk = itemgetter("k")
    return gk({"k": ka.get()}).run() + gk({"k": kb.get()}).run()


def use_ag(wa: Wrap, wb: WrapB) -> int:
    ga = attrgetter("a")
    return ga(wa.get()).run() + ga(wb.get()).run()


def use_ag_class() -> int:
    ga = attrgetter("a")
    return ga(Wrap(A())).run() + ga(WrapB(B())).run()


def use_class() -> int:
    gi = itemgetter(0)
    gk = itemgetter("k")
    return (
        gi([A()]).run()
        + gi([B()]).run()
        + gk({"k": A()}).run()
        + gk({"k": B()}).run()
    )


def use_preserves_b(pb: BoxB) -> int:
    gi = itemgetter(0)
    return gi([pb.get()]).run()
