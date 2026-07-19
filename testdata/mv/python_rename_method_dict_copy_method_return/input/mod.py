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


# Class regressions — already solid.
def use_class_dict_copy_sub() -> int:
    return (
        {"k": A()}.copy()["k"].run()
        + {"k": B()}.copy()["k"].run()
    )


def use_class_dict_copy_get() -> int:
    return (
        {"k": A()}.copy().get("k").run()
        + {"k": B()}.copy().get("k").run()
    )


def use_class_dict_copy_assign() -> int:
    da = {"k": A()}.copy()
    db = {"k": B()}.copy()
    return da["k"].run() + db["k"].run()


def use_class_dict_copy_nested() -> int:
    return (
        {"k": [A()]}.copy()["k"][0].run()
        + {"k": [B()]}.copy()["k"][0].run()
    )


def use_class_dict_plain() -> int:
    return (
        {"k": A()}["k"].run()
        + {"k": B()}["k"].run()
    )


# Method-return under foreign same-leaf.
def use_mr_dict_copy_sub(ba: BoxA, bb: BoxB) -> int:
    return (
        {"k": ba.get()}.copy()["k"].run()
        + {"k": bb.get()}.copy()["k"].run()
    )


def use_mr_dict_copy_get(ba: BoxA, bb: BoxB) -> int:
    return (
        {"k": ba.get()}.copy().get("k").run()
        + {"k": bb.get()}.copy().get("k").run()
    )


def use_mr_dict_copy_assign(ba: BoxA, bb: BoxB) -> int:
    da = {"k": ba.get()}.copy()
    db = {"k": bb.get()}.copy()
    return da["k"].run() + db["k"].run()


def use_mr_dict_copy_nested(ba: BoxA, bb: BoxB) -> int:
    return (
        {"k": [ba.get()]}.copy()["k"][0].run()
        + {"k": [bb.get()]}.copy()["k"][0].run()
    )


def use_mr_dict_plain(ba: BoxA, bb: BoxB) -> int:
    return (
        {"k": ba.get()}["k"].run()
        + {"k": bb.get()}["k"].run()
    )


# Preserves B.
def use_preserves_b(bb: BoxB) -> int:
    return {"k": bb.get()}.copy()["k"].run()
