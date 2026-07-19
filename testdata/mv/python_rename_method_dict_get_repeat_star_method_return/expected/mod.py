class A:
    def execute(self) -> int:
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


def use_dict_get_inline(ba: BoxA, bb: BoxB) -> int:
    return {"k": ba.get()}.get("k").execute() + {"k": bb.get()}.get("k").run()


def use_dict_get_assign(ba: BoxA, bb: BoxB) -> int:
    da = {"k": ba.get()}
    db = {"k": bb.get()}
    return da.get("k").execute() + db.get("k").run()


def use_dict_get_walrus(ba: BoxA, bb: BoxB) -> int:
    return (xa := {"k": ba.get()}.get("k")).execute() + (xb := {"k": bb.get()}.get("k")).run()


def use_dict_getitem(ba: BoxA, bb: BoxB) -> int:
    return {"k": ba.get()}.__getitem__("k").execute() + {"k": bb.get()}.__getitem__("k").run()


def use_list_repeat_inline(ba: BoxA, bb: BoxB) -> int:
    return ([ba.get()] * 2)[0].execute() + ([bb.get()] * 2)[0].run()


def use_list_repeat_left(ba: BoxA, bb: BoxB) -> int:
    return (2 * [ba.get()])[0].execute() + (2 * [bb.get()])[0].run()


def use_list_repeat_assign(ba: BoxA, bb: BoxB) -> int:
    xs = [ba.get()] * 2
    ys = [bb.get()] * 2
    return xs[0].execute() + ys[0].run()


def use_tuple_repeat(ba: BoxA, bb: BoxB) -> int:
    return ((ba.get(),) * 2)[0].execute() + ((bb.get(),) * 2)[0].run()


def use_dict_kw_inline(ba: BoxA, bb: BoxB) -> int:
    return dict(k=ba.get())["k"].execute() + dict(k=bb.get())["k"].run()


def use_dict_kw_get(ba: BoxA, bb: BoxB) -> int:
    return dict(k=ba.get()).get("k").execute() + dict(k=bb.get()).get("k").run()


def use_dict_kw_assign(ba: BoxA, bb: BoxB) -> int:
    da = dict(k=ba.get())
    db = dict(k=bb.get())
    return da["k"].execute() + db["k"].run()


def use_dict_kw_multi(ba: BoxA, bb: BoxB) -> int:
    return dict(k=ba.get(), j=ba.get())["j"].execute() + dict(k=bb.get(), j=bb.get())["j"].run()


def use_dict_from_literal(ba: BoxA, bb: BoxB) -> int:
    return dict({"k": ba.get()})["k"].execute() + dict({"k": bb.get()})["k"].run()


def use_star_list_inline(ba: BoxA, bb: BoxB) -> int:
    return [*[ba.get()]][0].execute() + [*[bb.get()]][0].run()


def use_star_list_mixed(ba: BoxA, bb: BoxB) -> int:
    return [ba.get(), *[ba.get()]][0].execute() + [bb.get(), *[bb.get()]][0].run()


def use_star_list_assign(ba: BoxA, bb: BoxB) -> int:
    xs = [*[ba.get()]]
    ys = [*[bb.get()]]
    return xs[0].execute() + ys[0].run()


def use_for_star_list(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for x in [*[ba.get()]]:
        n += x.execute()
    for y in [*[bb.get()]]:
        n += y.run()
    return n


def use_for_list_repeat(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for x in [ba.get()] * 2:
        n += x.execute()
    for y in [bb.get()] * 2:
        n += y.run()
    return n


def use_mixed_dict_kw(ba: BoxA, bb: BoxB) -> int:
    return dict(k=ba.get(), j=bb.get())["k"].run()


def use_mixed_star(ba: BoxA, bb: BoxB) -> int:
    return [ba.get(), *[bb.get()]][0].run()


def use_preserves_b(bb: BoxB) -> int:
    ys = [bb.get()] * 2
    db = dict(k=bb.get())
    return (
        {"k": bb.get()}.get("k").run()
        + dict(k=bb.get())["k"].run()
        + db["k"].run()
        + ([bb.get()] * 2)[0].run()
        + ys[0].run()
        + [*[bb.get()]][0].run()
    )
