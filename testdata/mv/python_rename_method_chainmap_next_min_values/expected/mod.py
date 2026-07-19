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


def use_chainmap_sub(ba: BoxA, bb: BoxB) -> int:
    return ChainMap({"k": ba.get()})["k"].execute() + ChainMap({"k": bb.get()})["k"].run()


def use_chainmap_get(ba: BoxA, bb: BoxB) -> int:
    return ChainMap({"k": ba.get()}).get("k").execute() + ChainMap({"k": bb.get()}).get("k").run()


def use_chainmap_assign(ba: BoxA, bb: BoxB) -> int:
    ca = ChainMap({"k": ba.get()})
    cb = ChainMap({"k": bb.get()})
    return ca["k"].execute() + cb["k"].run()


def use_chainmap_dict_kw(ba: BoxA, bb: BoxB) -> int:
    return ChainMap(dict(k=ba.get()))["k"].execute() + ChainMap(dict(k=bb.get()))["k"].run()


def use_chainmap_multi(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()}, {"j": ba.get()})["j"].execute()
        + ChainMap({"k": bb.get()}, {"j": bb.get()})["j"].run()
    )


def use_next_iter_list(ba: BoxA, bb: BoxB) -> int:
    return next(iter([ba.get()])).execute() + next(iter([bb.get()])).run()


def use_next_list(ba: BoxA, bb: BoxB) -> int:
    return next([ba.get()]).execute() + next([bb.get()]).run()


def use_next_assign(ba: BoxA, bb: BoxB) -> int:
    xa = next(iter([ba.get()]))
    xb = next(iter([bb.get()]))
    return xa.execute() + xb.run()


def use_next_walrus(ba: BoxA, bb: BoxB) -> int:
    return (xa := next(iter([ba.get()]))).execute() + (xb := next(iter([bb.get()]))).run()


def use_next_dict_values(ba: BoxA, bb: BoxB) -> int:
    return (
        next(iter(dict(k=ba.get()).values())).execute()
        + next(iter(dict(k=bb.get()).values())).run()
    )


def use_min_list(ba: BoxA, bb: BoxB) -> int:
    return min([ba.get()], key=lambda x: 0).execute() + min([bb.get()], key=lambda x: 0).run()


def use_max_list(ba: BoxA, bb: BoxB) -> int:
    return max([ba.get()], key=lambda x: 0).execute() + max([bb.get()], key=lambda x: 0).run()


def use_min_assign(ba: BoxA, bb: BoxB) -> int:
    xa = min([ba.get()], key=lambda x: 0)
    xb = min([bb.get()], key=lambda x: 0)
    return xa.execute() + xb.run()


def use_min_dict_values(ba: BoxA, bb: BoxB) -> int:
    return (
        min(dict(k=ba.get()).values(), key=lambda x: 0).execute()
        + min(dict(k=bb.get()).values(), key=lambda x: 0).run()
    )


def use_for_dict_values(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for x in dict(k=ba.get()).values():
        n += x.execute()
    for y in dict(k=bb.get()).values():
        n += y.run()
    return n


def use_for_literal_values(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for x in {"k": ba.get()}.values():
        n += x.execute()
    for y in {"k": bb.get()}.values():
        n += y.run()
    return n


def use_for_chainmap_values(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for x in ChainMap({"k": ba.get()}).values():
        n += x.execute()
    for y in ChainMap({"k": bb.get()}).values():
        n += y.run()
    return n


def use_for_list_method_return(ba: BoxA, bb: BoxB) -> int:
    n = 0
    for x in [ba.get()]:
        n += x.execute()
    for y in [bb.get()]:
        n += y.run()
    return n


def use_mixed_chainmap(ba: BoxA, bb: BoxB) -> int:
    return ChainMap({"k": ba.get()}, {"j": bb.get()})["k"].run()


def use_mixed_min(ba: BoxA, bb: BoxB) -> int:
    return min([ba.get(), bb.get()], key=lambda x: 0).run()


def use_preserves_b(bb: BoxB) -> int:
    ys = [bb.get()]
    cb = ChainMap({"k": bb.get()})
    return (
        ChainMap({"k": bb.get()})["k"].run()
        + cb["k"].run()
        + next(iter([bb.get()])).run()
        + min([bb.get()], key=lambda x: 0).run()
        + next(iter(dict(k=bb.get()).values())).run()
        + ys[0].run()
    )


def use_choice_list(ba: BoxA, bb: BoxB) -> int:
    return choice([ba.get()]).execute() + choice([bb.get()]).run()


def use_heappop_list(ba: BoxA, bb: BoxB) -> int:
    return heappop([ba.get()]).execute() + heappop([bb.get()]).run()


def use_reduce_list(ba: BoxA, bb: BoxB) -> int:
    return reduce(lambda a, b: a, [ba.get()]).execute() + reduce(lambda a, b: a, [bb.get()]).run()


def use_max_chainmap_values(ba: BoxA, bb: BoxB) -> int:
    return (
        max(ChainMap({"k": ba.get()}).values(), key=lambda x: 0).execute()
        + max(ChainMap({"k": bb.get()}).values(), key=lambda x: 0).run()
    )
