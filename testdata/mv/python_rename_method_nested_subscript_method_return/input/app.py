class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    def __init__(self) -> None:
        self.a = A()

    def get(self) -> A:
        return self.a


class BoxB:
    def __init__(self) -> None:
        self.b = B()

    def get(self) -> B:
        return self.b


# Class regression — already solid.
def use_class_nested_list() -> int:
    return [[A()]][0][0].run() + [[B()]][0][0].run()


def use_class_nested_tuple() -> int:
    return ((A(),),)[0][0].run() + ((B(),),)[0][0].run()


def use_class_list_of_dict() -> int:
    return [{"k": A()}][0]["k"].run() + [{"k": B()}][0]["k"].run()


def use_class_dict_of_list() -> int:
    return {"k": [A()]}["k"][0].run() + {"k": [B()]}["k"][0].run()


def use_class_dict_of_dict() -> int:
    return {"o": {"k": A()}}["o"]["k"].run() + {"o": {"k": B()}}["o"]["k"].run()


def use_class_assign_nested() -> int:
    xs = [[A()]]
    ys = [[B()]]
    return xs[0][0].run() + ys[0][0].run()


def use_class_assign_dict_nested() -> int:
    da = {"k": [A()]}
    db = {"k": [B()]}
    return da["k"][0].run() + db["k"][0].run()


# Method-return under foreign same-leaf.
def use_mr_nested_list(ba: BoxA, bb: BoxB) -> int:
    return [[ba.get()]][0][0].run() + [[bb.get()]][0][0].run()


def use_mr_nested_tuple(ba: BoxA, bb: BoxB) -> int:
    return ((ba.get(),),)[0][0].run() + ((bb.get(),),)[0][0].run()


def use_mr_list_of_dict(ba: BoxA, bb: BoxB) -> int:
    return [{"k": ba.get()}][0]["k"].run() + [{"k": bb.get()}][0]["k"].run()


def use_mr_dict_of_list(ba: BoxA, bb: BoxB) -> int:
    return {"k": [ba.get()]}["k"][0].run() + {"k": [bb.get()]}["k"][0].run()


def use_mr_dict_of_dict(ba: BoxA, bb: BoxB) -> int:
    return {"o": {"k": ba.get()}}["o"]["k"].run() + {"o": {"k": bb.get()}}["o"]["k"].run()


def use_mr_assign_nested(ba: BoxA, bb: BoxB) -> int:
    xs = [[ba.get()]]
    ys = [[bb.get()]]
    return xs[0][0].run() + ys[0][0].run()


def use_mr_assign_dict_nested(ba: BoxA, bb: BoxB) -> int:
    da = {"k": [ba.get()]}
    db = {"k": [bb.get()]}
    return da["k"][0].run() + db["k"][0].run()


def use_mr_assign_list_of_dict(ba: BoxA, bb: BoxB) -> int:
    la = [{"k": ba.get()}]
    lb = [{"k": bb.get()}]
    return la[0]["k"].run() + lb[0]["k"].run()


# Single-level already solid.
def use_mr_single(ba: BoxA, bb: BoxB) -> int:
    return [ba.get()][0].run() + [bb.get()][0].run()


def use_preserves_b(bb: BoxB) -> int:
    return (
        [[bb.get()]][0][0].run()
        + ((bb.get(),),)[0][0].run()
        + [{"k": bb.get()}][0]["k"].run()
        + {"k": [bb.get()]}["k"][0].run()
        + {"o": {"k": bb.get()}}["o"]["k"].run()
        + B().run()
    )
