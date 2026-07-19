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


def use_deque_popleft(ba: BoxA, bb: BoxB) -> int:
    return deque([ba.get()]).popleft().run() + deque([bb.get()]).popleft().run()


def use_deque_pop(ba: BoxA, bb: BoxB) -> int:
    return deque([ba.get()]).pop().run() + deque([bb.get()]).pop().run()


def use_list_pop(ba: BoxA, bb: BoxB) -> int:
    return [ba.get()].pop().run() + [bb.get()].pop().run()


def use_list_copy(ba: BoxA, bb: BoxB) -> int:
    return [ba.get()].copy()[0].run() + [bb.get()].copy()[0].run()


def use_list_slice(ba: BoxA, bb: BoxB) -> int:
    return [ba.get()][:][0].run() + [bb.get()][:][0].run()


def use_userlist(ba: BoxA, bb: BoxB) -> int:
    return UserList([ba.get()])[0].run() + UserList([bb.get()])[0].run()


def use_userlist_assign(ba: BoxA, bb: BoxB) -> int:
    xs = UserList([ba.get()])
    ys = UserList([bb.get()])
    return xs[0].run() + ys[0].run()


def use_dict_merge(ba: BoxA, bb: BoxB) -> int:
    return (
        ({"k": ba.get()} | {"j": ba.get()})["j"].run()
        + ({"k": bb.get()} | {"j": bb.get()})["j"].run()
    )


def use_dict_values_list(ba: BoxA, bb: BoxB) -> int:
    return (
        list({"k": ba.get()}.values())[0].run()
        + list({"k": bb.get()}.values())[0].run()
    )


def use_dict_values_list_dict(ba: BoxA, bb: BoxB) -> int:
    return (
        list(dict(k=ba.get()).values())[0].run()
        + list(dict(k=bb.get()).values())[0].run()
    )


def use_pop_assign(ba: BoxA, bb: BoxB) -> int:
    xa = [ba.get()].pop()
    xb = [bb.get()].pop()
    return xa.run() + xb.run()


def use_preserves_b(bb: BoxB) -> int:
    return (
        deque([bb.get()]).popleft().run()
        + [bb.get()].pop().run()
        + [bb.get()].copy()[0].run()
        + [bb.get()][:][0].run()
        + UserList([bb.get()])[0].run()
        + ({"k": bb.get()} | {"j": bb.get()})["j"].run()
        + list({"k": bb.get()}.values())[0].run()
    )
