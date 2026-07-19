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


def use_dict_inline(ba: BoxA, bb: BoxB) -> int:
    return {"k": ba.get()}["k"].execute() + {"k": bb.get()}["k"].run()


def use_dict_assign(ba: BoxA, bb: BoxB) -> int:
    da = {"k": ba.get()}
    db = {"k": bb.get()}
    return da["k"].execute() + db["k"].run()


def use_dict_multi(ba: BoxA, bb: BoxB) -> int:
    return {"k": ba.get(), "j": ba.get()}["j"].execute() + {"k": bb.get(), "j": bb.get()}["j"].run()


def use_dict_typed_local(ba: BoxA, bb: BoxB) -> int:
    xa = ba.get()
    xb = bb.get()
    return {"k": xa}["k"].execute() + {"k": xb}["k"].run()


def use_dict_walrus(ba: BoxA, bb: BoxB) -> int:
    return (da := {"k": ba.get()})["k"].execute() + (db := {"k": bb.get()})["k"].run()


def use_list_concat_inline(ba: BoxA, bb: BoxB) -> int:
    return ([ba.get()] + [ba.get()])[0].execute() + ([bb.get()] + [bb.get()])[0].run()


def use_list_concat_assign(ba: BoxA, bb: BoxB) -> int:
    xs = [ba.get()] + [ba.get()]
    ys = [bb.get()] + [bb.get()]
    return xs[0].execute() + ys[0].run()


def use_list_concat_empty(ba: BoxA, bb: BoxB) -> int:
    return ([] + [ba.get()])[0].execute() + ([] + [bb.get()])[0].run()


def use_list_concat_chain(ba: BoxA, bb: BoxB) -> int:
    return ([ba.get()] + [ba.get()] + [ba.get()])[0].execute() + ([bb.get()] + [bb.get()] + [bb.get()])[0].run()


def use_list_ctor_inline(ba: BoxA, bb: BoxB) -> int:
    return list([ba.get()])[0].execute() + list([bb.get()])[0].run()


def use_list_ctor_assign(ba: BoxA, bb: BoxB) -> int:
    xs = list([ba.get()])
    ys = list([bb.get()])
    return xs[0].execute() + ys[0].run()


def use_tuple_ctor(ba: BoxA, bb: BoxB) -> int:
    return tuple((ba.get(),))[0].execute() + tuple((bb.get(),))[0].run()


def use_list_or_inline(ba: BoxA, bb: BoxB) -> int:
    return ([ba.get()] or [ba.get()])[0].execute() + ([bb.get()] or [bb.get()])[0].run()


def use_list_or_assign(ba: BoxA, bb: BoxB) -> int:
    xs = [ba.get()] or [ba.get()]
    ys = [bb.get()] or [bb.get()]
    return xs[0].execute() + ys[0].run()


def use_list_and_inline(ba: BoxA, bb: BoxB) -> int:
    return ([ba.get()] and [ba.get()])[0].execute() + ([bb.get()] and [bb.get()])[0].run()


def use_list_or_empty(ba: BoxA, bb: BoxB) -> int:
    return ([ba.get()] or [])[0].execute() + ([bb.get()] or [])[0].run()


def use_for_list_ctor(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for x in list([ba.get()]):
        n += x.execute()
    for y in list([bb.get()]):
        n += y.run()
    return n


def use_for_list_concat(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for x in [ba.get()] + [ba.get()]:
        n += x.execute()
    for y in [bb.get()] + [bb.get()]:
        n += y.run()
    return n


def use_mixed_dict(ba: BoxA, bb: BoxB) -> int:
    return {"k": ba.get(), "j": bb.get()}["k"].run()


def use_mixed_concat(ba: BoxA, bb: BoxB) -> int:
    return ([ba.get()] + [bb.get()])[0].run()


def use_preserves_b(bb: BoxB) -> int:
    ys = [bb.get()] + [bb.get()]
    db = {"k": bb.get()}
    return (
        {"k": bb.get()}["k"].run()
        + db["k"].run()
        + ([bb.get()] + [bb.get()])[0].run()
        + ys[0].run()
        + list([bb.get()])[0].run()
        + ([bb.get()] or [bb.get()])[0].run()
    )
