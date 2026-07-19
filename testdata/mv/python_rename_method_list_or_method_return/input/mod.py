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

    def self(self) -> "BoxA":
        return self


class BoxB:
    b: B

    def __init__(self, b: B):
        self.b = b

    def get(self) -> B:
        return self.b

    def self(self) -> "BoxB":
        return self


def use_list_inline(ba: BoxA, bb: BoxB) -> int:
    return [ba.get()][0].run() + [bb.get()][0].run()


def use_list_assign(ba: BoxA, bb: BoxB) -> int:
    xs = [ba.get()]
    ys = [bb.get()]
    return xs[0].run() + ys[0].run()


def use_list_multi(ba: BoxA, bb: BoxB) -> int:
    return [ba.get(), ba.get()][1].run() + [bb.get(), bb.get()][1].run()


def use_tuple_inline(ba: BoxA, bb: BoxB) -> int:
    return (ba.get(),)[0].run() + (bb.get(),)[0].run()


def use_tuple_assign(ba: BoxA, bb: BoxB) -> int:
    xs = (ba.get(),)
    ys = (bb.get(),)
    return xs[0].run() + ys[0].run()


def use_list_chain(ba: BoxA, bb: BoxB) -> int:
    return [ba.self().get()][0].run() + [bb.self().get()][0].run()


def use_list_typed_local(ba: BoxA, bb: BoxB) -> int:
    xa = ba.get()
    xb = bb.get()
    return [xa][0].run() + [xb][0].run()


def use_list_walrus(ba: BoxA, bb: BoxB) -> int:
    return (xs := [ba.get()])[0].run() + (ys := [bb.get()])[0].run()


def use_list_ternary(ba: BoxA, bb: BoxB, c: bool) -> int:
    return [(ba.get() if c else ba.get())][0].run() + [(bb.get() if c else bb.get())][0].run()


def use_or_method(ba: BoxA, bb: BoxB) -> int:
    return (ba.get() or ba.get()).run() + (bb.get() or bb.get()).run()


def use_and_method(ba: BoxA, bb: BoxB) -> int:
    return (ba.get() and ba.get()).run() + (bb.get() and bb.get()).run()


def use_or_chain(ba: BoxA, bb: BoxB) -> int:
    return (ba.get() or ba.get() or ba.get()).run() + (bb.get() or bb.get() or bb.get()).run()


def use_or_typed(ba: BoxA, bb: BoxB) -> int:
    xa = ba.get()
    xb = bb.get()
    return (xa or xa).run() + (xb or xb).run()


def use_and_typed(ba: BoxA, bb: BoxB) -> int:
    xa = ba.get()
    xb = bb.get()
    return (xa and xa).run() + (xb and xb).run()


def use_for_list(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for x in [ba.get()]:
        n += x.run()
    for y in [bb.get()]:
        n += y.run()
    return n


def use_mixed_or(ba: BoxA, bb: BoxB) -> int:
    return (ba.get() or bb.get()).run()


def use_or_none(ba: BoxA) -> int:
    return (ba.get() or None).run()


def use_preserves_b(bb: BoxB) -> int:
    ys = [bb.get()]
    return (
        [bb.get()][0].run()
        + ys[0].run()
        + (bb.get() or bb.get()).run()
        + (bb.get() and bb.get()).run()
    )
