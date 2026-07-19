from collections import namedtuple


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    a: A

    def __init__(self, a: A) -> None:
        self.a = a

    def get(self) -> A:
        return self.a


class BoxB:
    b: B

    def __init__(self, b: B) -> None:
        self.b = b

    def get(self) -> B:
        return self.b


# Unique namedtuple types per side (dual-class isolation).
NTA = namedtuple("NTA", ["x"])
NTB = namedtuple("NTB", ["y"])


# --- Class regressions: namedtuple _replace / _make (already solid). ---
def use_class_replace_inline() -> int:
    return NTA(A())._replace(x=A()).x.run() + NTB(B())._replace(y=B()).y.run()


def use_class_replace_local() -> int:
    na = NTA(A())
    nb = NTB(B())
    return na._replace(x=A()).x.run() + nb._replace(y=B()).y.run()


def use_class_replace_assign() -> int:
    na = NTA(B())
    nb = NTB(A())
    ra = na._replace(x=A())
    rb = nb._replace(y=B())
    return ra.x.run() + rb.y.run()


def use_class_make_inline() -> int:
    return NTA._make([A()]).x.run() + NTB._make([B()]).y.run()


def use_class_make_index() -> int:
    return NTA._make([A()])[0].run() + NTB._make([B()])[0].run()


def use_class_make_assign() -> int:
    na = NTA._make([A()])
    nb = NTB._make([B()])
    return na.x.run() + nb.y.run()


def use_class_make_tuple() -> int:
    return NTA._make((A(),)).x.run() + NTB._make((B(),)).y.run()


# --- Method-return under foreign same-leaf. ---
def use_mr_replace_inline(ba: BoxA, bb: BoxB) -> int:
    return (
        NTA(ba.get())._replace(x=ba.get()).x.run()
        + NTB(bb.get())._replace(y=bb.get()).y.run()
    )


def use_mr_replace_local(ba: BoxA, bb: BoxB) -> int:
    na = NTA(A())
    nb = NTB(B())
    return na._replace(x=ba.get()).x.run() + nb._replace(y=bb.get()).y.run()


def use_mr_replace_assign(ba: BoxA, bb: BoxB) -> int:
    na = NTA(B())
    nb = NTB(A())
    ra = na._replace(x=ba.get())
    rb = nb._replace(y=bb.get())
    return ra.x.run() + rb.y.run()


def use_mr_make_inline(ba: BoxA, bb: BoxB) -> int:
    return NTA._make([ba.get()]).x.run() + NTB._make([bb.get()]).y.run()


def use_mr_make_index(ba: BoxA, bb: BoxB) -> int:
    return NTA._make([ba.get()])[0].run() + NTB._make([bb.get()])[0].run()


def use_mr_make_assign(ba: BoxA, bb: BoxB) -> int:
    na = NTA._make([ba.get()])
    nb = NTB._make([bb.get()])
    return na.x.run() + nb.y.run()


def use_mr_make_tuple(ba: BoxA, bb: BoxB) -> int:
    return NTA._make((ba.get(),)).x.run() + NTB._make((bb.get(),)).y.run()


# Plain namedtuple ctor field access (already solid).
def use_mr_plain(ba: BoxA, bb: BoxB) -> int:
    return NTA(ba.get()).x.run() + NTB(bb.get()).y.run()


def use_class_plain() -> int:
    return NTA(A()).x.run() + NTB(B()).y.run()


def use_preserves_b(bb: BoxB) -> int:
    return (
        NTB(bb.get())._replace(y=bb.get()).y.run()
        + NTB._make([bb.get()]).y.run()
        + NTB._make([bb.get()])[0].run()
    )
