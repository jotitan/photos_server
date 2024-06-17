import React, {useEffect, useRef, useState} from "react";
import './timeline.css';

export default function Timeline({images, setImages}) {
    const [selected, setSelected] = useState(-1);
    const [nameFolders, setNameFolders] = useState([]);
    const [sizeBlock, setSizeBlock] = useState(100);
    const [groupedImages, setGroupedImages] = useState([]);
    const [left, setLeft] = useState(0)
    const refDiv = useRef();
    useEffect(() => {
        const listFolders = images.reduce((m, img) => {
            const key = img.folder.substring(0, img.folder.length - 1);
            if (!m.has(key)) {
                m.set(key, [img])
            } else {
                m.get(key).push(img)
            }
            return m
        }, new Map());
        const entries = [...listFolders.entries()].sort((a,b)=>a[1][0] >b[1][0] ? -1:1)

        const names = entries.map(e=>e[0]).map(f => f.replaceAll('_',' ').substring(f.lastIndexOf("/") + 1).toLowerCase());
        const newSizeBlock = names.reduce((max,w)=>Math.max(max,w.length),0) * 8;
        setSizeBlock(newSizeBlock)
        document.querySelector(":root").style.setProperty('--width-timeline-black',`${newSizeBlock}px`)
        setNameFolders(names);
        setGroupedImages(entries.map(e=>e[1]));
    }, [images])

    useEffect(() => {
        if(selected === -1){
            return;
        }
        setImages(groupedImages[selected])
        //eslint-disable-next-line react-hooks/exhaustive-deps
    }, [selected])


    const step = sizeBlock * 2;
    const goRight = () => {
        // Stop if end of div and not displayable
        const totalSize = nameFolders.length * sizeBlock;
        if(totalSize + left < refDiv.current.offsetWidth){
            return;
        }
        setLeft(left - step);
    }

    const goLeft = () => {
        if (left >= 0) {
            return;
        }
        setLeft(left + step);
    }

    return <div className={"my-timeline"}>

        <div onClick={goLeft}> &#x3c; </div>
        <div ref={refDiv}>
            <div style={{transition:'1s',left: `${left}px`}}>
                <ol>
                    {nameFolders.map((f, i) => <li style={{left: `${i * sizeBlock}px`}}>
                        <span className={"text"} onClick={() => setSelected(i)}>{f}</span>
                        <span className={`bullet ${i === selected ? 'selected' : ''}`}
                              onClick={() => setSelected(i)}></span>
                    </li>)}
                </ol>
            </div>
        </div>
        <div onClick={goRight} style={{left: '96%', position: 'absolute'}}> &#x3e; </div>
        <span className={"line-bullets"}></span>
    </div>
}

