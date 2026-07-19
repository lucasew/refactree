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


def use_walrus_expr(ba: BoxA, bb: BoxB) -> int:
    return (a := ba.get()).run() + (b := bb.get()).run()


def use_walrus_chain(ba: BoxA, bb: BoxB) -> int:
    return (xa := ba.self()).get().run() + (xb := bb.self()).get().run()


def use_walrus_bind(ba: BoxA, bb: BoxB) -> int:
    total = 0
    if (a := ba.get()):
        total += a.run()
    if (b := bb.get()):
        total += b.run()
    return total


def use_ternary_method(ba: BoxA, bb: BoxB, c: bool) -> int:
    return (ba.get() if c else ba.get()).run() + (bb.get() if c else bb.get()).run()


def use_ternary_nested(ba: BoxA, bb: BoxB, c: bool) -> int:
    return (ba.self().get() if c else ba.get()).run() + (bb.self().get() if c else bb.get()).run()


def use_ternary_recv(ba: BoxA, bb: BoxB, c: bool) -> int:
    return (ba if c else ba).get().run() + (bb if c else bb).get().run()


def use_paren_chain(ba: BoxA, bb: BoxB) -> int:
    return (ba.self()).get().run() + (bb.self()).get().run()


def use_mixed(ba: BoxA, bb: BoxB, c: bool) -> int:
    return (ba.get() if c else bb.get()).run()


def use_preserves_b(bb: BoxB, c: bool) -> int:
    return (
        (b := bb.get()).run()
        + (bb.get() if c else bb.get()).run()
        + (bb if c else bb).get().run()
        + (xb := bb.self()).get().run()
    )
