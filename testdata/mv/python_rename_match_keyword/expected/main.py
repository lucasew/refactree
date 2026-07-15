class Box:
    assist: int
    stay: int

    def __init__(self, helper, stay):
        self.assist = helper
        self.stay = stay


def use(x):
    match x:
        case Box(assist=h, stay=s):
            return h + s
        case _:
            return 0
