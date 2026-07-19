from collections import ChainMap


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_nested_new_child():
    da: dict[str, list[A]] = {"k": [A()]}
    db: dict[str, list[B]] = {"k": [B()]}
    ca = ChainMap(da).new_child({"j": [A()]})
    cb = ChainMap(db).new_child({"j": [B()]})
    return ca["j"][0].run() + cb["j"][0].run()


def use_scalar_new_child():
    da: dict[str, A] = {"k": A()}
    db: dict[str, B] = {"k": B()}
    ca = ChainMap(da).new_child({"j": A()})
    cb = ChainMap(db).new_child({"j": B()})
    return ca["j"].run() + cb["j"].run()


def use_empty_new_child():
    da: dict[str, A] = {"k": A()}
    db: dict[str, B] = {"k": B()}
    ca = ChainMap(da).new_child()
    cb = ChainMap(db).new_child()
    return ca["k"].run() + cb["k"].run()


def use_inline_new_child():
    da: dict[str, list[A]] = {"k": [A()]}
    db: dict[str, list[B]] = {"k": [B()]}
    return (
        ChainMap(da).new_child({"j": [A()]})["j"][0].run()
        + ChainMap(db).new_child({"j": [B()]})["j"][0].run()
    )
