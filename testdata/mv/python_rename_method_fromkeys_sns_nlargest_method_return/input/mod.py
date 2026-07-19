from types import SimpleNamespace
from heapq import nlargest


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


def use_fromkeys_sub(ba: BoxA, bb: BoxB) -> int:
    return (
        dict.fromkeys(["k"], ba.get())["k"].run()
        + dict.fromkeys(["k"], bb.get())["k"].run()
    )


def use_fromkeys_values(ba: BoxA, bb: BoxB) -> int:
    return (
        list(dict.fromkeys(["k"], ba.get()).values())[0].run()
        + list(dict.fromkeys(["k"], bb.get()).values())[0].run()
    )


def use_fromkeys_assign(ba: BoxA, bb: BoxB) -> int:
    da = dict.fromkeys(["k"], ba.get())
    db = dict.fromkeys(["k"], bb.get())
    return next(iter(da.values())).run() + next(iter(db.values())).run()


def use_sns_assign(ba: BoxA, bb: BoxB) -> int:
    da = SimpleNamespace(k=ba.get())
    db = SimpleNamespace(k=bb.get())
    return da.k.run() + db.k.run()


def use_sns_inline(ba: BoxA, bb: BoxB) -> int:
    return (
        SimpleNamespace(k=ba.get()).k.run()
        + SimpleNamespace(k=bb.get()).k.run()
    )


def use_filter(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for a in filter(None, [ba.get()]):
        n += a.run()
    for b in filter(None, [bb.get()]):
        n += b.run()
    return n


def use_nlargest(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for a in nlargest(1, [ba.get()], key=lambda x: 0):
        n += a.run()
    for b in nlargest(1, [bb.get()], key=lambda x: 0):
        n += b.run()
    return n


def use_preserves_b(bb: BoxB) -> int:
    db = SimpleNamespace(k=bb.get())
    n = 0
    for b in filter(None, [bb.get()]):
        n += b.run()
    return (
        n
        + dict.fromkeys(["k"], bb.get())["k"].run()
        + db.k.run()
        + SimpleNamespace(k=bb.get()).k.run()
    )
