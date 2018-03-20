---
hidden:     false
layout:     post
title:      为什么要用线程池而不是Java Thread API
date:       2018-03-21 11:21:29
summary:    资源，业务需求
categories: tech
---

 - 资源，线程在Java里面并不是很小的开销
 - 业务需求。new Thread并不能保证线程执行顺序，但是线程池可以，默认是FIFO